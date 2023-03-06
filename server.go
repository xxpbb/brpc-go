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
	"net"
	"reflect"
	"sync"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
	"google.golang.org/protobuf/proto"

	"github.com/xxpbb/brpc-go/brpcpb"
)

type methodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error)

// MethodDesc represents an RPC service's method specification.
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}

// ServiceDesc represents an RPC service's specification.
type ServiceDesc struct {
	ServiceName string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
}

// serviceInfo wraps information about a service. It is very similar to
// ServiceDesc and is constructed from it for internal purposes.
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl interface{}
	methods     map[string]*MethodDesc
}

// Server is a bRPC server to serve RPC requests.
type Server struct {
	opts serverOptions

	mu        sync.Mutex // guards following
	serve     bool
	eventLoop netpoll.EventLoop
	cv        *sync.Cond              // signaled when connections close for GracefulStop
	services  map[string]*serviceInfo // service name -> service info
}

type serverOptions struct {
}

var defaultServerOptions = serverOptions{}

// A ServerOption sets options such as credentials, codec and keepalive parameters, etc.
type ServerOption interface {
	apply(*serverOptions)
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
// type funcServerOption struct {
// 	f func(*serverOptions)
// }

// func (fdo *funcServerOption) apply(do *serverOptions) {
// 	fdo.f(do)
// }

// func newFuncServerOption(f func(*serverOptions)) *funcServerOption {
// 	return &funcServerOption{
// 		f: f,
// 	}
// }

// NewServer creates a bRPC server which has no service registered and has not
// started to accept requests yet.
func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	s := &Server{
		opts:     opts,
		services: make(map[string]*serviceInfo),
	}
	s.cv = sync.NewCond(&s.mu)
	return s
}

// ServiceRegistrar wraps a single method that supports service registration. It
// enables users to pass concrete types other than brpc.Server to the service
// registration methods exported by the IDL generated code.
type ServiceRegistrar interface {
	// RegisterService registers a service and its implementation to the
	// concrete type implementing this interface.  It may not be called
	// once the server has started serving.
	// desc describes the service and its methods and handlers. impl is the
	// service implementation which is passed to the method handlers.
	RegisterService(desc *ServiceDesc, impl interface{})
}

// RegisterService registers a service and its implementation to the bRPC
// server. It is called from the IDL generated code. This must be called before
// invoking Serve. If ss is non-nil (for legacy code), its type is checked to
// ensure it implements sd.HandlerType.
func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			log.Fatalf("brpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
	}
	s.register(sd, ss)
}

func (s *Server) register(sd *ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serve {
		log.Fatalf("brpc: Server.RegisterService after Server.Serve for %q", sd.ServiceName)
	}
	if _, ok := s.services[sd.ServiceName]; ok {
		log.Fatalf("brpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*MethodDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	s.services[sd.ServiceName] = info
}

func (s *Server) Stop() {
	s.GracefulStop()
}

func (s *Server) GracefulStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventLoop.Shutdown(context.Background())

	s.serve = false
}

// Serve accepts incoming connections on the listener lis, creating a new
// ServerTransport and service goroutine for each. The service goroutines
// read bRPC requests and then call the registered handlers to reply to them.
// Serve returns when lis.Accept fails with fatal errors.  lis will be closed when
// this method returns.
// Serve will return a non-nil error unless Stop or GracefulStop is called.
func (s *Server) Serve(lis net.Listener) error {
	s.serve = true

	eventLoop, err := netpoll.NewEventLoop(s.connRequestCb, netpoll.WithOnPrepare(s.connPrepareCb))
	if err != nil {
		return err
	}

	s.eventLoop = eventLoop

	return eventLoop.Serve(lis)
}

type wqContextKey struct{}

var gWQContextKey = &wqContextKey{}

func (s *Server) connPrepareCb(conn netpoll.Connection) context.Context {
	wq := mux.NewShardQueue(mux.ShardSize, conn)

	conn.AddCloseCallback(func(c netpoll.Connection) error {
		log.Printf("conn [%s] is closed\n", conn.RemoteAddr())
		return wq.Close()
	})

	return context.WithValue(context.Background(), gWQContextKey, wq)
}

func (s *Server) connRequestCb(ctx context.Context, conn netpoll.Connection) error {
	wq, ok := ctx.Value(gWQContextKey).(*mux.ShardQueue)
	if !ok {
		return fmt.Errorf("wq not found")
	}

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

	gopool.Go(func() { s.handleMsg(r, wq) })

	return nil
}

func (s *Server) handleMsg(reader netpoll.Reader, wq *mux.ShardQueue) error {
	// decode request
	m := NewMessage()

	err := m.Decode(reader)
	if err != nil {
		return err
	}

	serviceName := m.meta.GetRequest().GetServiceName()
	methodName := m.meta.GetRequest().GetMethodName()
	id := m.meta.GetCorrelationId()

	// handle
	cntl := &Controller{
		requestAttachment: m.attachment,
	}
	srvInfo := s.services[serviceName]
	out, err := srvInfo.methods[methodName].Handler(srvInfo.serviceImpl, newContextWithController(cntl), func(in interface{}) error {
		return proto.Unmarshal(m.data, in.(proto.Message))
	})
	if err != nil {
		return err
	}

	// encode response
	data, err := proto.Marshal(out.(proto.Message))
	if err != nil {
		return err
	}

	m = &Message{
		meta: &brpcpb.RpcMeta{
			Response: &brpcpb.RpcResponseMeta{
				ErrorCode: proto.Int32(0),
			},
			CorrelationId:  proto.Int64(id),
			AttachmentSize: proto.Int32(int32(len(cntl.responseAttachment))),
		},
		data:       data,
		attachment: cntl.responseAttachment,
	}

	w, err := m.Encode()
	if err != nil {
		return err
	}

	// send
	wq.Add(func() (netpoll.Writer, bool) {
		return w, false
	})

	return nil
}
