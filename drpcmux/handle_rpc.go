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
func (m *Mux) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	data, ok := m.rpcs[rpc]
	if !ok {
		return drpc.ProtocolError.New("unknown rpc: %q", rpc)
	}

	in := interface{}(stream)
	if data.in1 != streamType {
		msg, ok := reflect.New(data.in1.Elem()).Interface().(drpc.Message)
		if !ok {
			return drpc.InternalError.New("invalid rpc input type")
		}
		if err := stream.MsgRecv(msg, data.enc); err != nil {
			return errs.Wrap(err)
		}
		in = msg
	}

	var out drpc.Message
	if data.in1 != streamType && m.unaryInterceptor != nil {
		out, err = m.unaryInterceptor(stream.Context(), in, rpc,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				return data.receiver(data.srv, ctx, req, stream)
			})
	} else if data.in1 == streamType && m.streamInterceptor != nil {
		out, err = m.streamInterceptor(stream, rpc,
			func(st drpc.Stream) (interface{}, error) {
				return data.receiver(data.srv, st.Context(), st, stream)
			})
	} else {
		out, err = data.receiver(data.srv, stream.Context(), in, stream)
	}

	switch {
	case err != nil:
		return errs.Wrap(err)
	case out != nil && !reflect.ValueOf(out).IsNil():
		return stream.MsgSend(out, data.enc)
	default:
		return stream.CloseSend()
	}
}
