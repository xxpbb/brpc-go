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

// Package status implements errors returned by bRPC.  These errors are
// serialized and transmitted on the wire between server and client, and allow
// for additional data to be transmitted via the Details field in the status
// proto.  bRPC service handlers should return an error created by this
// package, and bRPC clients should expect a corresponding error to be
// returned from the RPC call.
//
// This package upholds the invariants that a non-nil error may not
// contain an OK code, and an OK code must result in a nil error.
package status

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/xxpbb/brpc-go/brpcpb"
)

// Status represents an RPC status code and message. It is immutable
// and should be created with New, Newf, or FromProto.
type Status struct {
	s *brpcpb.RpcResponseMeta
}

// New returns a Status representing c and msg.
func New(c int32, msg string) *Status {
	return &Status{s: &brpcpb.RpcResponseMeta{ErrorCode: proto.Int32(c), ErrorText: proto.String(msg)}}
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c int32, format string, a ...interface{}) *Status {
	return New(c, fmt.Sprintf(format, a...))
}

// FromProto returns a Status representing s.
func FromProto(s *brpcpb.RpcResponseMeta) *Status {
	return &Status{s: proto.Clone(s).(*brpcpb.RpcResponseMeta)}
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c int32, msg string) error {
	return New(c, msg).Err()
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c int32, format string, a ...interface{}) error {
	return Error(c, fmt.Sprintf(format, a...))
}

// ErrorProto returns an error representing s.  If s.Code is OK, returns nil.
func ErrorProto(s *brpcpb.RpcResponseMeta) error {
	return FromProto(s).Err()
}

// FromError returns a Status representation of err.
//
//   - If err was produced by this package or implements the method `BRPCStatus()
//     *Status`, the appropriate Status is returned.
//
//   - If err is nil, a Status is returned with codes.OK and no message.
//
//   - Otherwise, err is an error not compatible with this package.  In this
//     case, a Status is returned with codes.Unknown and err's Error() message,
//     and ok is false.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}
	if se, ok := err.(interface {
		BRPCStatus() *Status
	}); ok {
		return se.BRPCStatus(), true
	}
	return New(-1, err.Error()), false
}

// Convert is a convenience function which removes the need to handle the
// boolean return value from FromError.
func Convert(err error) *Status {
	s, _ := FromError(err)
	return s
}

// Code returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise.
func Code(err error) int32 {
	// Don't use FromError to avoid allocation of OK status.
	if err == nil {
		return 0
	}
	if se, ok := err.(interface {
		BRPCStatus() *Status
	}); ok {
		return se.BRPCStatus().Code()
	}
	return -1
}

// FromContextError converts a context error or wrapped context error into a
// Status.  It returns a Status with codes.OK if err is nil, or a Status with
// codes.Unknown if err is non-nil and not a context error.
func FromContextError(err error) *Status {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return New(int32(brpcpb.Errno_ERPCTIMEDOUT), err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return New(int32(brpcpb.Errno_EREQUEST), err.Error())
	}
	return New(-1, err.Error())
}

// Code returns the status code contained in s.
func (s *Status) Code() int32 {
	if s == nil || s.s == nil {
		return 0
	}
	return *s.s.ErrorCode
}

// Message returns the message contained in s.
func (s *Status) Message() string {
	if s == nil || s.s == nil {
		return ""
	}
	return *s.s.ErrorText
}

// Proto returns s's status as an spb.Status proto message.
func (s *Status) Proto() *brpcpb.RpcResponseMeta {
	if s == nil {
		return nil
	}
	return proto.Clone(s.s).(*brpcpb.RpcResponseMeta)
}

// Err returns an immutable error representing s; returns nil if s.Code() is OK.
func (s *Status) Err() error {
	if s.Code() == 0 {
		return nil
	}
	return &StatusError{s: s}
}

func (s *Status) String() string {
	return fmt.Sprintf("rpc error: code = %d desc = %s", s.Code(), s.Message())
}

// StatusError wraps a pointer of a status proto. It implements error and Status,
// and a nil *StatusError should never be returned by this package.
type StatusError struct {
	s *Status
}

func (e *StatusError) Error() string {
	return e.s.String()
}

// BRPCStatus returns the Status represented by se.
func (e *StatusError) BRPCStatus() *Status {
	return e.s
}

// Is implements future error.Is functionality.
// A Error is equivalent if the code and message are identical.
func (e *StatusError) Is(target error) bool {
	tse, ok := target.(*StatusError)
	if !ok {
		return false
	}
	return proto.Equal(e.s.s, tse.s.s)
}
