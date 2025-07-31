// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmux

import (
	"context"
	"reflect"

	"github.com/zeebo/errs"
	"storj.io/drpc"
)

// HandleRPC handles the rpc that has been requested by the stream.
func (m *Mux) HandleRPC(originalStream drpc.Stream, rpc string) (err error) {
	data, ok := m.rpcs[rpc]
	if !ok {
		return drpc.ProtocolError.New("unknown rpc: %q", rpc)
	}

	in := interface{}(originalStream)
	var out drpc.Message

	// Select interceptor based on RPC type. Note: data.unitary means if the RPC
	// has unary input and output.
	switch {
	case data.unitary && m.unaryInterceptor != nil:
		// Unary interceptor for unary RPCs (unary -> unary)
		in, err = m.msgRecv(data, originalStream)
		if err != nil {
			break
		}
		out, err = m.unaryInterceptor(originalStream.Context(), in, rpc,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				return data.receiver(data.srv, ctx, req, originalStream)
			})
	case !data.unitary && m.streamInterceptor != nil:
		if data.in1 == streamType {
			// Stream input case (stream -> stream or stream -> unary)
			out, err = m.streamInterceptor(originalStream, rpc,
				func(stream drpc.Stream) (interface{}, error) {
					return data.receiver(data.srv, stream.Context(), stream, stream)
				})
		} else {
			// Unary input with stream output (unary -> stream)
			out, err = m.streamInterceptor(originalStream, rpc,
				func(stream drpc.Stream) (interface{}, error) {
					// Get input message from the stream
					input, err := m.msgRecv(data, stream)
					if err != nil {
						return nil, err
					}
					return data.receiver(data.srv, stream.Context(), input, stream)
				})
		}
	default:
		// Default case with no interceptors.
		if data.in1 != streamType {
			// For unary input RPCs with either unary or stream output
			if in, err = m.msgRecv(data, originalStream); err != nil {
				break
			}
		}
		out, err = data.receiver(data.srv, originalStream.Context(), in, originalStream)
	}

	switch {
	case err != nil:
		return errs.Wrap(err)
	case out != nil && !reflect.ValueOf(out).IsNil():
		return originalStream.MsgSend(out, data.enc)
	default:
		return originalStream.CloseSend()
	}
}

// msgRecv receives a message from the stream
func (m *Mux) msgRecv(data rpcData, stream drpc.Stream) (drpc.Message, error) {
	msg, ok := reflect.New(data.in1.Elem()).Interface().(drpc.Message)
	if !ok {
		return msg, drpc.InternalError.New("invalid rpc input type")
	}
	err := stream.MsgRecv(msg, data.enc)
	return msg, errs.Wrap(err)
}
