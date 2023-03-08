/*
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package brpc

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
	"google.golang.org/protobuf/proto"

	"github.com/xxpbb/brpc-go/brpcpb"
)

// ClientConnInterface defines the functions clients need to perform unary RPCs.
// It is implemented by *clientConn, and is only intended to be referenced by generated code.
type ClientConnInterface interface {
	Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...CallOption) error
}

// Assert *clientConn implements ClientConn.
var _ ClientConnInterface = (*ClientConn)(nil)

// ClientConn represents a virtual connection to a conceptual endpoint, to
// perform RPCs.
//
// A ClientConn is free to have zero or more actual connections to the endpoint
// based on configuration, load, etc. It is also free to determine which actual
// endpoints to use and may change it every RPC, permitting client-side load
// balancing.
//
// A ClientConn encapsulates a range of functionality including name
// resolution, TCP connection establishment (with retries and backoff) and TLS
// handshakes. It also handles errors on established connections by
// re-resolving the name and reconnecting.
type ClientConn struct {
	// The following are initialized at dial time, and are read-only after that.
	target string      // User's dial target.
	dopts  dialOptions // Default and user specified dial options.

	// The following provide their own synchronization, and therefore don't
	// require cc.mu to be held to access them.
	correlationId atomic.Int64
	pendingRpcs   sync.Map

	// mu protects the following fields.
	mu   sync.Mutex
	conn netpoll.Connection // Set to nil on close.
	wq   *mux.ShardQueue
}

// Dial creates a client connection to the given target.
func Dial(target string, opts ...DialOption) (*ClientConn, error) {
	return DialContext(context.Background(), target, opts...)
}

// DialContext creates a client connection to the given target. By default, it's
// a non-blocking dial (the function won't wait for connections to be
// established, and connecting happens in the background). To make it a blocking
// dial, use WithBlock() dial option.
//
// In the non-blocking case, the ctx does not act against the connection. It
// only controls the setup steps.
//
// In the blocking case, ctx can be used to cancel or expire the pending
// connection. Once this function returns, the cancellation and expiration of
// ctx will be noop. Users should call ClientConn.Close to terminate all the
// pending operations after this function returns.
//
// The target name syntax is defined in
// https://github.com/grpc/grpc/blob/master/doc/naming.md.
// e.g. to use dns resolver, a "dns:///" prefix should be applied to the target.
func DialContext(ctx context.Context, target string, opts ...DialOption) (*ClientConn, error) {
	cc := &ClientConn{
		target:        target,
		dopts:         defaultDialOptions(),
		correlationId: atomic.Int64{},
		pendingRpcs:   sync.Map{},
		mu:            sync.Mutex{},
		conn:          nil,
		wq:            nil,
	}

	for _, opt := range opts {
		opt.apply(&cc.dopts)
	}

	// A blocking dial blocks until the clientConn is ready.
	err := cc.Connect()
	if err != nil {
		return nil, err
	}

	return cc, nil
}

// Invoke sends the RPC request on the wire and returns after response is
// received.  This is typically called by generated code.
//
// All errors returned by Invoke are compatible with the status package.
func (cc *ClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...CallOption) error {
	// allow interceptor to see all applicable call options, which means those
	// configured as defaults from dial option as well as per-call options
	opts = combine(cc.dopts.callOptions, opts)

	copts := callOptions{}
	for _, opt := range opts {
		opt.apply(&copts)
	}

	requestAttachment := copts.requestAttachment
	responseAttachment := copts.responseAttachment

	id := cc.correlationId.Add(1)

	names := strings.Split(method, "/")

	// reconnect
	if cc.conn == nil || !cc.conn.IsActive() {
		if err := cc.Connect(); err != nil {
			return err
		}
	}

	// send
	err := cc.sendMsg(names[1], names[2], id, args.(proto.Message), requestAttachment)
	if err != nil {
		return err
	}

	// recv
	err = cc.recvMsg(ctx, id, reply.(proto.Message), responseAttachment)
	if err != nil {
		return err
	}

	return nil
}

func combine(o1 []CallOption, o2 []CallOption) []CallOption {
	// we don't use append because o1 could have extra capacity whose
	// elements would be overwritten, which could cause inadvertent
	// sharing (and race conditions) between concurrent calls
	if len(o1) == 0 {
		return o2
	} else if len(o2) == 0 {
		return o1
	}
	ret := make([]CallOption, len(o1)+len(o2))
	copy(ret, o1)
	copy(ret[len(o1):], o2)
	return ret
}

// Connect causes all subchannels in the ClientConn to attempt to connect if
// the channel is idle.
func (cc *ClientConn) Connect() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.conn != nil && cc.conn.IsActive() {
		return nil
	}

	c, err := netpoll.DialConnection("tcp", cc.target, cc.dopts.timeout)
	if err != nil {
		return err
	}
	cc.conn = c
	cc.wq = mux.NewShardQueue(mux.ShardSize, cc.conn)

	cc.conn.AddCloseCallback(func(conn netpoll.Connection) error {
		log.Printf("conn [%s] is closed\n", cc.target)
		return cc.wq.Close()
	})

	cc.conn.SetOnRequest(func(ctx context.Context, conn netpoll.Connection) error {
		reader := conn.Reader()

		headerB, err := reader.Peek(8)
		if err != nil {
			return err
		}
		bodyL := int(binary.BigEndian.Uint32(headerB[4:8]))

		r, err := reader.Slice(12 + bodyL)
		if err != nil {
			return err
		}

		gopool.Go(func() { cc.handleMsg(r) })

		return nil
	})

	return nil
}

// Close tears down the ClientConn and all underlying connections.
func (cc *ClientConn) Close() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.conn != nil {
		err := cc.conn.Close()
		if err != nil {
			return err
		}
		cc.conn = nil
	}

	return nil
}

func (cc *ClientConn) handleMsg(reader netpoll.Reader) error {
	m := NewMessage()

	err := m.Decode(reader)
	if err != nil {
		return err
	}

	id := m.meta.GetCorrelationId()

	ch, ok := cc.pendingRpcs.Load(id)
	if !ok {
		return fmt.Errorf("received %d, but not found", id)
	}
	replyCh := ch.(chan *Message)

	replyCh <- m

	return nil
}

func (cc *ClientConn) recvMsg(ctx context.Context, id int64, reply proto.Message, attachment *[]byte) error {
	defer cc.pendingRpcs.Delete(id)

	ch, ok := cc.pendingRpcs.Load(id)
	if !ok {
		return fmt.Errorf("rpc %d not found", id)
	}
	replyCh := ch.(chan *Message)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-replyCh:
			err := proto.Unmarshal(m.data, reply)
			if err != nil {
				return err
			}

			if m.meta.GetAttachmentSize() > 0 && attachment != nil {
				*attachment = m.attachment
			}

			return nil
		}
	}
}

func (cc *ClientConn) sendMsg(serviceName string, methodName string, id int64, in proto.Message, attachment []byte) error {
	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	msg := &Message{
		meta: &brpcpb.RpcMeta{
			Request: &brpcpb.RpcRequestMeta{
				ServiceName: proto.String(serviceName),
				MethodName:  proto.String(methodName),
			},
			CorrelationId:  proto.Int64(id),
			AttachmentSize: proto.Int32(int32(len(attachment))),
		},
		data:       data,
		attachment: attachment,
	}

	w, err := msg.Encode()
	if err != nil {
		return err
	}

	// send
	cc.wq.Add(func() (netpoll.Writer, bool) {
		return w, false
	})

	cc.pendingRpcs.Store(id, make(chan *Message))

	return nil
}
