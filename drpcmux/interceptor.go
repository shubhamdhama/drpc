package drpcmux

import (
	"context"

	"storj.io/drpc"
)

// UnaryHandler defines the handler for the unary RPC.
type UnaryHandler func(ctx context.Context, req interface{}) (out interface{}, err error)

// UnaryServerInterceptor defines the server side interceptor for unary RPC.
type UnaryServerInterceptor func(
	ctx context.Context, req interface{}, rpc string, handler UnaryHandler) (out interface{}, err error)

func chainUnaryInterceptors(interceptors []UnaryServerInterceptor) UnaryServerInterceptor {
	switch n := len(interceptors); n {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	default:
		return func(ctx context.Context, req interface{}, rpc string, handler UnaryHandler) (
			out interface{}, err error,
		) {
			return interceptors[0](
				ctx, req, rpc, getChainedUnaryHandler(interceptors, 1, rpc, handler),
			)
		}
	}
}

func getChainedUnaryHandler(
	interceptors []UnaryServerInterceptor, currIdx int, rpc string, handler UnaryHandler,
) UnaryHandler {
	if currIdx == len(interceptors) {
		return handler
	}
	return func(ctx context.Context, req interface{}) (out interface{}, err error) {
		return interceptors[currIdx](
			ctx, req, rpc, getChainedUnaryHandler(interceptors, currIdx+1, rpc, handler),
		)
	}
}

// StreamHandler defines the handler for the stream RPC.
type StreamHandler func(stream drpc.Stream) (out interface{}, err error)

// StreamServerInterceptor defines a server side interceptor for unary RPC.
type StreamServerInterceptor func(
	stream drpc.Stream, rpc string, handler StreamHandler) (out interface{}, err error)

func chainStreamInterceptors(interceptors []StreamServerInterceptor) StreamServerInterceptor {
	switch n := len(interceptors); n {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	default:
		return func(stream drpc.Stream, rpc string, handler StreamHandler) (
			out interface{}, err error,
		) {
			return interceptors[0](
				stream, rpc, getChainedStreamHandler(interceptors, 1, rpc, handler),
			)
		}
	}
}

func getChainedStreamHandler(
	interceptors []StreamServerInterceptor, currIdx int, rpc string, handler StreamHandler,
) StreamHandler {
	if currIdx == len(interceptors) {
		return handler
	}
	return func(stream drpc.Stream) (out interface{}, err error) {
		return interceptors[currIdx](
			stream, rpc, getChainedStreamHandler(interceptors, currIdx+1, rpc, handler),
		)
	}
}
