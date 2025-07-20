package drpcmux

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/drpc"
)

// mockRPC interface for registering with the mux
type mockRPC interface {
	// RPC with unary input and unary output
	mockMethod1(ctx context.Context, in *mockMessage) (*mockMessage, error)
	// RPC with unary input and stream output
	mockMethod2(in *mockMessage, stream drpc.Stream) error
	// RPC with stream input
	mockMethod3(stream drpc.Stream) error
}

// mockRPCImpl is a mock implementation that we'll register with the mux
type mockRPCImpl struct {
	t              *testing.T
	expectedMethod string
	expectedIn     *mockMessage
	outMsg         *mockMessage
	returnErr      error
}

func (m *mockRPCImpl) mockMethod(_ context.Context, in *mockMessage) (*mockMessage, error) {
	// Verify that we received what we expected
	if in.Value != m.expectedIn.Value {
		m.t.Errorf("expected input '%s', got '%s'", m.expectedIn.Value, in.Value)
	}

	return m.outMsg, m.returnErr
}

func (m *mockRPCImpl) mockStreamMethod(_ drpc.Stream) error {
	return m.returnErr
}

// mockDescription implements drpc.Description for testing
type mockDescription struct {
	rpcName   string
	encoding  mockEncoding
	receiver  drpc.Receiver
	method    interface{}
	methodNum int
}

func (m mockDescription) NumMethods() int {
	return m.methodNum
}

func (m mockDescription) Method(
	n int,
) (rpc string, encoding drpc.Encoding, receiver drpc.Receiver, method interface{}, ok bool) {
	if n >= m.methodNum {
		return "", nil, nil, nil, false
	}
	return m.rpcName, m.encoding, m.receiver, m.method, true
}

// TestHandleRPCWithUnaryInterceptor tests that unary interceptors are executed
// during HandleRPC
func TestHandleRPCWithUnaryInterceptor(t *testing.T) {
	r := require.New(t)
	interceptorCalled := false
	modifiedResponse := &mockMessage{Value: "modified_response"}

	interceptor := func(
		ctx context.Context, req interface{}, rpc string, handler UnaryHandler,
	) (interface{}, error) {
		interceptorCalled = true
		r.Equal("request", req.(*mockMessage).Value)
		r.Equal("test.Method", rpc)

		// Call the handler but return our modified response
		_, err := handler(ctx, req)
		return modifiedResponse, err
	}

	mux := NewWithInterceptors([]UnaryServerInterceptor{interceptor}, nil)

	// Create the implementation and register it with the mux
	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.Method",
		expectedIn:     &mockMessage{Value: "request"},
		outMsg:         &mockMessage{Value: "original_response"},
		returnErr:      nil,
	}

	receiver := func(
		srv interface{}, ctx context.Context, in1, in2 interface{},
	) (drpc.Message, error) {
		// Make sure we got the right service implementation
		r.Equal(impl, srv)
		return impl.mockMethod(ctx, in1.(*mockMessage))
	}

	desc := mockDescription{
		rpcName:   "test.Method",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "request"},
	}

	err = mux.HandleRPC(stream, "test.Method")
	r.NoError(err)
	r.True(interceptorCalled, "interceptor was not called")

	sentMsg, ok := stream.sendMsg.(*mockMessage)
	r.True(ok, "expected *mockMessage")
	r.Equal("modified_response", sentMsg.Value)
}

// TestHandleRPCWithStreamInterceptor tests that stream interceptors are
// executed during HandleRPC
func TestHandleRPCWithStreamInterceptor(t *testing.T) {
	r := require.New(t)
	interceptorCalled := false

	interceptor := func(
		stream drpc.Stream, rpc string, handler StreamHandler,
	) (interface{}, error) {
		interceptorCalled = true
		r.Equal("test.StreamMethod", rpc)

		return handler(stream)
	}

	mux := NewWithInterceptors(nil, []StreamServerInterceptor{interceptor})

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.StreamMethod",
		returnErr:      nil,
	}

	receiver := func(
		srv interface{}, ctx context.Context, in1, in2 interface{},
	) (drpc.Message, error) {
		r.Equal(impl, srv)
		r.Equal(in1, in2, "in1 and in2 should be the same for stream method")

		err := impl.mockStreamMethod(in1.(drpc.Stream))
		return nil, err
	}

	desc := mockDescription{
		rpcName:   "test.StreamMethod",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod3,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	// Create the mock stream
	stream := &mockStream{
		ctx: context.Background(),
	}

	// Handle the RPC
	err = mux.HandleRPC(stream, "test.StreamMethod")

	// Check results
	r.NoError(err)
	r.True(interceptorCalled, "interceptor was not called")
}

// TestHandleRPCWithErrorInInterceptor tests error handling in interceptors
func TestHandleRPCWithErrorInInterceptor(t *testing.T) {
	r := require.New(t)
	expectedError := errors.New("interceptor error")

	interceptor := func(
		ctx context.Context, req interface{}, rpc string, handler UnaryHandler,
	) (interface{}, error) {
		return nil, expectedError
	}

	mux := NewWithInterceptors([]UnaryServerInterceptor{interceptor}, nil)

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.Method",
		expectedIn:     &mockMessage{Value: "request"},
		outMsg:         &mockMessage{Value: "response"},
		returnErr:      nil,
	}

	receiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		// This should not be called because the interceptor returns an error
		r.Fail("receiver was called, but interceptor should have aborted the call")
		return impl.mockMethod(ctx, in1.(*mockMessage))
	}

	desc := mockDescription{
		rpcName:   "test.Method",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "request"},
	}

	err = mux.HandleRPC(stream, "test.Method")

	r.Error(err)
	r.ErrorIs(err, expectedError)
}

// TestHandleRPCWithMultipleInterceptors tests chaining multiple interceptors
// during HandleRPC
func TestHandleRPCWithMultipleInterceptors(t *testing.T) {
	r := require.New(t)
	var order []string

	interceptor1 := func(ctx context.Context, req interface{}, rpc string, handler UnaryHandler) (interface{}, error) {
		order = append(order, "interceptor1_before")
		resp, err := handler(ctx, req)
		order = append(order, "interceptor1_after")
		return resp, err
	}

	interceptor2 := func(ctx context.Context, req interface{}, rpc string, handler UnaryHandler) (interface{}, error) {
		order = append(order, "interceptor2_before")
		resp, err := handler(ctx, req)
		order = append(order, "interceptor2_after")
		return resp, err
	}

	mux := NewWithInterceptors([]UnaryServerInterceptor{interceptor1, interceptor2}, nil)

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.Method",
		expectedIn:     &mockMessage{Value: "request"},
		outMsg:         &mockMessage{Value: "response"},
		returnErr:      nil,
	}

	receiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		order = append(order, "handler")
		return impl.mockMethod(ctx, in1.(*mockMessage))
	}

	desc := mockDescription{
		rpcName:   "test.Method",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "request"},
	}

	err = mux.HandleRPC(stream, "test.Method")
	r.NoError(err)

	expectedOrder := []string{
		"interceptor1_before",
		"interceptor2_before",
		"handler",
		"interceptor2_after",
		"interceptor1_after",
	}

	r.Equal(expectedOrder, order)
}

// modifyingStream wraps a stream and modifies incoming messages
type modifyingStream struct {
	drpc.Stream
	prefix string
}

// MsgRecv overrides the Stream.MsgRecv to modify the message after receiving it
func (m *modifyingStream) MsgRecv(msg drpc.Message, enc drpc.Encoding) error {
	err := m.Stream.MsgRecv(msg, enc)
	if err != nil {
		return err
	}

	// Modify the message after receiving it
	if mockMsg, ok := msg.(*mockMessage); ok {
		mockMsg.Value = m.prefix + mockMsg.Value
	}
	return nil
}

// TestHandleRPCWithUnaryInputStreamOutput tests that stream interceptors
// are executed correctly when an RPC has unary input and stream output
func TestHandleRPCWithUnaryInputStreamOutput(t *testing.T) {
	r := require.New(t)
	interceptorCalled := false
	messageModified := false

	interceptor := func(
		stream drpc.Stream, rpc string, handler StreamHandler,
	) (interface{}, error) {
		interceptorCalled = true
		r.Equal("test.UnaryStreamMethod", rpc)

		// Wrap the stream with our modifying stream
		modifiedStream := &modifyingStream{
			Stream: stream,
			prefix: "MODIFIED_",
		}

		return handler(modifiedStream)
	}

	mux := NewWithInterceptors(nil, []StreamServerInterceptor{interceptor})

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.UnaryStreamMethod",
		returnErr:      nil,
	}

	// This receiver will handle a method with unary input and stream output
	receiver := func(
		srv interface{}, ctx context.Context, in1, in2 interface{},
	) (drpc.Message, error) {
		r.Equal(impl, srv)

		// Verify that in1 has been modified by the stream interceptor
		inputMsg, ok := in1.(*mockMessage)
		r.True(ok, "expected *mockMessage for in1, got %T", in1)

		if inputMsg.Value == "MODIFIED_original_input" {
			messageModified = true
		} else {
			r.Failf("expected modified input value 'MODIFIED_original_input', got '%s'", inputMsg.Value)
		}

		return nil, impl.returnErr
	}

	// Create a mock description that represents a method with unary input and stream output
	desc := mockDescription{
		rpcName:   "test.UnaryStreamMethod",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod2, // mockMethod2 has unary input, stream output signature
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	// Create the mock stream with our unary input message
	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "original_input"},
	}

	// Handle the RPC
	err = mux.HandleRPC(stream, "test.UnaryStreamMethod")

	// Check results
	r.NoError(err)
	r.True(interceptorCalled, "stream interceptor was not called for unary input/stream output method")
	r.True(messageModified, "stream interceptor did not modify the message content")
}

// TestHandleRPCWithMixedInterceptors tests that the appropriate interceptors are
// selected based on the RPC type
func TestHandleRPCWithMixedInterceptors(t *testing.T) {
	r := require.New(t)
	unaryInterceptorCalled := false
	streamInterceptorCalled := false

	unaryInterceptor := func(
		ctx context.Context, req interface{}, rpc string, handler UnaryHandler,
	) (interface{}, error) {
		unaryInterceptorCalled = true
		return handler(ctx, req)
	}

	streamInterceptor := func(
		stream drpc.Stream, rpc string, handler StreamHandler,
	) (interface{}, error) {
		streamInterceptorCalled = true
		return handler(stream)
	}

	// Register both types of interceptors in the same mux
	mux := NewWithInterceptors(
		[]UnaryServerInterceptor{unaryInterceptor},
		[]StreamServerInterceptor{streamInterceptor},
	)

	impl := &mockRPCImpl{t: t}

	// Register multiple methods with different input/output types

	// 1. Unary input/output method
	unaryReceiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		return nil, nil
	}

	// 2. Stream
	streamReceiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		return nil, nil
	}

	// 3. Unary input, stream output method
	unaryStreamReceiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		return nil, nil
	}

	// Register all methods
	err := mux.Register(impl, mockDescription{
		rpcName:   "test.UnaryMethod",
		encoding:  mockEncoding{},
		receiver:  unaryReceiver,
		method:    mockRPC.mockMethod1, // unary input, unary output
		methodNum: 1,
	})
	r.NoError(err, "failed to register unary method")

	err = mux.Register(impl, mockDescription{
		rpcName:   "test.StreamMethod",
		encoding:  mockEncoding{},
		receiver:  streamReceiver,
		method:    mockRPC.mockMethod3, // stream input
		methodNum: 1,
	})
	r.NoError(err, "failed to register stream method")

	err = mux.Register(impl, mockDescription{
		rpcName:   "test.UnaryStreamMethod",
		encoding:  mockEncoding{},
		receiver:  unaryStreamReceiver,
		method:    mockRPC.mockMethod2, // unary input, stream output
		methodNum: 1,
	})
	r.NoError(err, "failed to register unary-stream method")

	// Test 1: Call unary method - should use unary interceptor
	stream1 := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "test_input"},
	}

	unaryInterceptorCalled = false
	streamInterceptorCalled = false

	err = mux.HandleRPC(stream1, "test.UnaryMethod")
	r.NoError(err, "unary method call failed")
	r.True(unaryInterceptorCalled, "unary interceptor was not called for unary method")
	r.False(streamInterceptorCalled, "stream interceptor was incorrectly called for unary method")

	// Test 2: Call stream method - should use stream interceptor
	stream2 := &mockStream{
		ctx: context.Background(),
	}

	unaryInterceptorCalled = false
	streamInterceptorCalled = false

	err = mux.HandleRPC(stream2, "test.StreamMethod")
	r.NoError(err, "stream method call failed")
	r.False(unaryInterceptorCalled, "unary interceptor was incorrectly called for stream method")
	r.True(streamInterceptorCalled, "stream interceptor was not called for stream method")

	// Test 3: Call unary input / stream output method - should use stream interceptor
	stream3 := &mockStream{
		ctx:     context.Background(),
		recvMsg: &mockMessage{Value: "test_input"},
	}

	unaryInterceptorCalled = false
	streamInterceptorCalled = false

	err = mux.HandleRPC(stream3, "test.UnaryStreamMethod")
	r.NoError(err, "unary-stream method call failed")
	r.False(unaryInterceptorCalled, "unary interceptor was incorrectly called for unary-stream method")
	r.True(streamInterceptorCalled, "stream interceptor was not called for unary-stream method")
}

// TestHandleRPCWithoutInterceptors tests that RPCs work correctly when no interceptors
// are defined
func TestHandleRPCWithoutInterceptors(t *testing.T) {
	r := require.New(t)

	// Create a mux with no interceptors
	mux := New()

	// Setup test data
	expectedInput := &mockMessage{Value: "test_input"}
	expectedOutput := &mockMessage{Value: "test_output"}

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.UnaryMethod",
		expectedIn:     expectedInput,
		outMsg:         expectedOutput,
		returnErr:      nil,
	}

	receiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		return impl.mockMethod(ctx, in1.(*mockMessage))
	}

	desc := mockDescription{
		rpcName:   "test.UnaryMethod",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: expectedInput,
	}

	err = mux.HandleRPC(stream, "test.UnaryMethod")

	// Check results
	r.NoError(err)

	// Check that the output message was sent correctly
	sentMsg, ok := stream.sendMsg.(*mockMessage)
	r.True(ok, "expected *mockMessage")
	r.Equal(expectedOutput.Value, sentMsg.Value)
}

// TestHandleRPCWithInvalidMethod tests behavior when trying to call a non-existent RPC method
func TestHandleRPCWithInvalidMethod(t *testing.T) {
	r := require.New(t)

	mux := New()

	// Register a valid method
	impl := &mockRPCImpl{}

	receiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		return nil, nil
	}

	desc := mockDescription{
		rpcName:   "test.ValidMethod",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1,
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx: context.Background(),
	}

	// Try to call a non-existent method
	err = mux.HandleRPC(stream, "test.NonExistentMethod")

	// Should return a protocol error
	r.Error(err, "expected error for non-existent method")
	r.True(drpc.ProtocolError.Has(err), "expected protocol error")
}

// TestHandleRPCWithUnaryRPCStreamInterceptor tests that unary RPCs bypass stream interceptors
// when only stream interceptors are present
func TestHandleRPCWithUnaryRPCStreamInterceptor(t *testing.T) {
	r := require.New(t)
	streamInterceptorCalled := false

	streamInterceptor := func(
		stream drpc.Stream, rpc string, handler StreamHandler,
	) (interface{}, error) {
		streamInterceptorCalled = true
		return handler(stream)
	}

	// Create mux with only stream interceptor (no unary interceptor)
	mux := NewWithInterceptors(nil, []StreamServerInterceptor{streamInterceptor})

	// Set up a unary input/unary output method
	expectedInput := &mockMessage{Value: "test_input"}
	expectedOutput := &mockMessage{Value: "test_output"}

	impl := &mockRPCImpl{
		t:              t,
		expectedMethod: "test.UnaryMethod",
		expectedIn:     expectedInput,
		outMsg:         expectedOutput,
		returnErr:      nil,
	}

	receiver := func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
		// Verify we got the right input
		inputMsg, ok := in1.(*mockMessage)
		r.True(ok, "expected *mockMessage for in1")
		r.Equal(expectedInput.Value, inputMsg.Value)

		return impl.mockMethod(ctx, in1.(*mockMessage))
	}

	desc := mockDescription{
		rpcName:   "test.UnaryMethod",
		encoding:  mockEncoding{},
		receiver:  receiver,
		method:    mockRPC.mockMethod1, // unary input, unary output
		methodNum: 1,
	}

	err := mux.Register(impl, desc)
	r.NoError(err, "failed to register")

	stream := &mockStream{
		ctx:     context.Background(),
		recvMsg: expectedInput,
	}

	err = mux.HandleRPC(stream, "test.UnaryMethod")

	// Check results
	r.NoError(err)
	r.False(streamInterceptorCalled, "stream interceptor should not be called for unary method when no unary interceptor is available")

	// Verify output was sent correctly
	sentMsg, ok := stream.sendMsg.(*mockMessage)
	r.True(ok, "expected *mockMessage")
	r.Equal(expectedOutput.Value, sentMsg.Value)
}
