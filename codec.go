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
	"encoding/binary"

	"github.com/cloudwego/netpoll"
	"google.golang.org/protobuf/proto"

	"github.com/xxpbb/brpc-go/brpcpb"
)

var (
	magicHead = []byte("PRPC")
)

type Message struct {
	meta       *brpcpb.RpcMeta
	data       []byte
	attachment []byte
}

func NewMessage() *Message {
	return &Message{meta: &brpcpb.RpcMeta{}}
}

func (m *Message) Encode() (netpoll.Writer, error) {
	meta, err := proto.Marshal(m.meta)
	if err != nil {
		return nil, err
	}
	metaL := len(meta)

	bodyL := metaL + len(m.data) + len(m.attachment)

	writer := netpoll.NewLinkBuffer()

	// header
	header, _ := writer.Malloc(12)
	copy(header, magicHead)
	binary.BigEndian.PutUint32(header[4:], uint32(bodyL))
	binary.BigEndian.PutUint32(header[8:], uint32(metaL))

	// body
	_, err = writer.WriteBinary(meta)
	if err != nil {
		return nil, err
	}
	_, err = writer.WriteBinary(m.data)
	if err != nil {
		return nil, err
	}
	_, err = writer.WriteBinary(m.attachment)
	if err != nil {
		return nil, err
	}

	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return writer, nil
}

func (m *Message) Decode(reader netpoll.Reader) error {
	// header
	headerB, err := reader.ReadBinary(12)
	if err != nil {
		return err
	}
	bodyL := int(binary.BigEndian.Uint32(headerB[4:8]))
	metaL := int(binary.BigEndian.Uint32(headerB[8:12]))

	// meta
	meta, err := reader.ReadBinary(metaL)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(meta, m.meta)
	if err != nil {
		return err
	}

	// attachment length
	attachmentL := int(m.meta.GetAttachmentSize())

	// data length
	dataL := bodyL - metaL - attachmentL

	// data
	m.data, err = reader.ReadBinary(dataL)
	if err != nil {
		return err
	}

	// attachment
	m.attachment, err = reader.ReadBinary(attachmentL)
	if err != nil {
		return err
	}

	return reader.Release()
}
