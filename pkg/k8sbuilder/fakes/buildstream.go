// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"context"
	"sync"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"
	"google.golang.org/grpc/metadata"
)

type BuildStream struct {
	ContextStub        func() context.Context
	contextMutex       sync.RWMutex
	contextArgsForCall []struct {
	}
	contextReturns struct {
		result1 context.Context
	}
	contextReturnsOnCall map[int]struct {
		result1 context.Context
	}
	RecvMsgStub        func(interface{}) error
	recvMsgMutex       sync.RWMutex
	recvMsgArgsForCall []struct {
		arg1 interface{}
	}
	recvMsgReturns struct {
		result1 error
	}
	recvMsgReturnsOnCall map[int]struct {
		result1 error
	}
	SendStub        func(*sidecar.BuildResponse) error
	sendMutex       sync.RWMutex
	sendArgsForCall []struct {
		arg1 *sidecar.BuildResponse
	}
	sendReturns struct {
		result1 error
	}
	sendReturnsOnCall map[int]struct {
		result1 error
	}
	SendHeaderStub        func(metadata.MD) error
	sendHeaderMutex       sync.RWMutex
	sendHeaderArgsForCall []struct {
		arg1 metadata.MD
	}
	sendHeaderReturns struct {
		result1 error
	}
	sendHeaderReturnsOnCall map[int]struct {
		result1 error
	}
	SendMsgStub        func(interface{}) error
	sendMsgMutex       sync.RWMutex
	sendMsgArgsForCall []struct {
		arg1 interface{}
	}
	sendMsgReturns struct {
		result1 error
	}
	sendMsgReturnsOnCall map[int]struct {
		result1 error
	}
	SetHeaderStub        func(metadata.MD) error
	setHeaderMutex       sync.RWMutex
	setHeaderArgsForCall []struct {
		arg1 metadata.MD
	}
	setHeaderReturns struct {
		result1 error
	}
	setHeaderReturnsOnCall map[int]struct {
		result1 error
	}
	SetTrailerStub        func(metadata.MD)
	setTrailerMutex       sync.RWMutex
	setTrailerArgsForCall []struct {
		arg1 metadata.MD
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *BuildStream) Context() context.Context {
	fake.contextMutex.Lock()
	ret, specificReturn := fake.contextReturnsOnCall[len(fake.contextArgsForCall)]
	fake.contextArgsForCall = append(fake.contextArgsForCall, struct {
	}{})
	stub := fake.ContextStub
	fakeReturns := fake.contextReturns
	fake.recordInvocation("Context", []interface{}{})
	fake.contextMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) ContextCallCount() int {
	fake.contextMutex.RLock()
	defer fake.contextMutex.RUnlock()
	return len(fake.contextArgsForCall)
}

func (fake *BuildStream) ContextCalls(stub func() context.Context) {
	fake.contextMutex.Lock()
	defer fake.contextMutex.Unlock()
	fake.ContextStub = stub
}

func (fake *BuildStream) ContextReturns(result1 context.Context) {
	fake.contextMutex.Lock()
	defer fake.contextMutex.Unlock()
	fake.ContextStub = nil
	fake.contextReturns = struct {
		result1 context.Context
	}{result1}
}

func (fake *BuildStream) ContextReturnsOnCall(i int, result1 context.Context) {
	fake.contextMutex.Lock()
	defer fake.contextMutex.Unlock()
	fake.ContextStub = nil
	if fake.contextReturnsOnCall == nil {
		fake.contextReturnsOnCall = make(map[int]struct {
			result1 context.Context
		})
	}
	fake.contextReturnsOnCall[i] = struct {
		result1 context.Context
	}{result1}
}

func (fake *BuildStream) RecvMsg(arg1 interface{}) error {
	fake.recvMsgMutex.Lock()
	ret, specificReturn := fake.recvMsgReturnsOnCall[len(fake.recvMsgArgsForCall)]
	fake.recvMsgArgsForCall = append(fake.recvMsgArgsForCall, struct {
		arg1 interface{}
	}{arg1})
	stub := fake.RecvMsgStub
	fakeReturns := fake.recvMsgReturns
	fake.recordInvocation("RecvMsg", []interface{}{arg1})
	fake.recvMsgMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) RecvMsgCallCount() int {
	fake.recvMsgMutex.RLock()
	defer fake.recvMsgMutex.RUnlock()
	return len(fake.recvMsgArgsForCall)
}

func (fake *BuildStream) RecvMsgCalls(stub func(interface{}) error) {
	fake.recvMsgMutex.Lock()
	defer fake.recvMsgMutex.Unlock()
	fake.RecvMsgStub = stub
}

func (fake *BuildStream) RecvMsgArgsForCall(i int) interface{} {
	fake.recvMsgMutex.RLock()
	defer fake.recvMsgMutex.RUnlock()
	argsForCall := fake.recvMsgArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) RecvMsgReturns(result1 error) {
	fake.recvMsgMutex.Lock()
	defer fake.recvMsgMutex.Unlock()
	fake.RecvMsgStub = nil
	fake.recvMsgReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) RecvMsgReturnsOnCall(i int, result1 error) {
	fake.recvMsgMutex.Lock()
	defer fake.recvMsgMutex.Unlock()
	fake.RecvMsgStub = nil
	if fake.recvMsgReturnsOnCall == nil {
		fake.recvMsgReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.recvMsgReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) Send(arg1 *sidecar.BuildResponse) error {
	fake.sendMutex.Lock()
	ret, specificReturn := fake.sendReturnsOnCall[len(fake.sendArgsForCall)]
	fake.sendArgsForCall = append(fake.sendArgsForCall, struct {
		arg1 *sidecar.BuildResponse
	}{arg1})
	stub := fake.SendStub
	fakeReturns := fake.sendReturns
	fake.recordInvocation("Send", []interface{}{arg1})
	fake.sendMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) SendCallCount() int {
	fake.sendMutex.RLock()
	defer fake.sendMutex.RUnlock()
	return len(fake.sendArgsForCall)
}

func (fake *BuildStream) SendCalls(stub func(*sidecar.BuildResponse) error) {
	fake.sendMutex.Lock()
	defer fake.sendMutex.Unlock()
	fake.SendStub = stub
}

func (fake *BuildStream) SendArgsForCall(i int) *sidecar.BuildResponse {
	fake.sendMutex.RLock()
	defer fake.sendMutex.RUnlock()
	argsForCall := fake.sendArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) SendReturns(result1 error) {
	fake.sendMutex.Lock()
	defer fake.sendMutex.Unlock()
	fake.SendStub = nil
	fake.sendReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SendReturnsOnCall(i int, result1 error) {
	fake.sendMutex.Lock()
	defer fake.sendMutex.Unlock()
	fake.SendStub = nil
	if fake.sendReturnsOnCall == nil {
		fake.sendReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.sendReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SendHeader(arg1 metadata.MD) error {
	fake.sendHeaderMutex.Lock()
	ret, specificReturn := fake.sendHeaderReturnsOnCall[len(fake.sendHeaderArgsForCall)]
	fake.sendHeaderArgsForCall = append(fake.sendHeaderArgsForCall, struct {
		arg1 metadata.MD
	}{arg1})
	stub := fake.SendHeaderStub
	fakeReturns := fake.sendHeaderReturns
	fake.recordInvocation("SendHeader", []interface{}{arg1})
	fake.sendHeaderMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) SendHeaderCallCount() int {
	fake.sendHeaderMutex.RLock()
	defer fake.sendHeaderMutex.RUnlock()
	return len(fake.sendHeaderArgsForCall)
}

func (fake *BuildStream) SendHeaderCalls(stub func(metadata.MD) error) {
	fake.sendHeaderMutex.Lock()
	defer fake.sendHeaderMutex.Unlock()
	fake.SendHeaderStub = stub
}

func (fake *BuildStream) SendHeaderArgsForCall(i int) metadata.MD {
	fake.sendHeaderMutex.RLock()
	defer fake.sendHeaderMutex.RUnlock()
	argsForCall := fake.sendHeaderArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) SendHeaderReturns(result1 error) {
	fake.sendHeaderMutex.Lock()
	defer fake.sendHeaderMutex.Unlock()
	fake.SendHeaderStub = nil
	fake.sendHeaderReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SendHeaderReturnsOnCall(i int, result1 error) {
	fake.sendHeaderMutex.Lock()
	defer fake.sendHeaderMutex.Unlock()
	fake.SendHeaderStub = nil
	if fake.sendHeaderReturnsOnCall == nil {
		fake.sendHeaderReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.sendHeaderReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SendMsg(arg1 interface{}) error {
	fake.sendMsgMutex.Lock()
	ret, specificReturn := fake.sendMsgReturnsOnCall[len(fake.sendMsgArgsForCall)]
	fake.sendMsgArgsForCall = append(fake.sendMsgArgsForCall, struct {
		arg1 interface{}
	}{arg1})
	stub := fake.SendMsgStub
	fakeReturns := fake.sendMsgReturns
	fake.recordInvocation("SendMsg", []interface{}{arg1})
	fake.sendMsgMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) SendMsgCallCount() int {
	fake.sendMsgMutex.RLock()
	defer fake.sendMsgMutex.RUnlock()
	return len(fake.sendMsgArgsForCall)
}

func (fake *BuildStream) SendMsgCalls(stub func(interface{}) error) {
	fake.sendMsgMutex.Lock()
	defer fake.sendMsgMutex.Unlock()
	fake.SendMsgStub = stub
}

func (fake *BuildStream) SendMsgArgsForCall(i int) interface{} {
	fake.sendMsgMutex.RLock()
	defer fake.sendMsgMutex.RUnlock()
	argsForCall := fake.sendMsgArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) SendMsgReturns(result1 error) {
	fake.sendMsgMutex.Lock()
	defer fake.sendMsgMutex.Unlock()
	fake.SendMsgStub = nil
	fake.sendMsgReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SendMsgReturnsOnCall(i int, result1 error) {
	fake.sendMsgMutex.Lock()
	defer fake.sendMsgMutex.Unlock()
	fake.SendMsgStub = nil
	if fake.sendMsgReturnsOnCall == nil {
		fake.sendMsgReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.sendMsgReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SetHeader(arg1 metadata.MD) error {
	fake.setHeaderMutex.Lock()
	ret, specificReturn := fake.setHeaderReturnsOnCall[len(fake.setHeaderArgsForCall)]
	fake.setHeaderArgsForCall = append(fake.setHeaderArgsForCall, struct {
		arg1 metadata.MD
	}{arg1})
	stub := fake.SetHeaderStub
	fakeReturns := fake.setHeaderReturns
	fake.recordInvocation("SetHeader", []interface{}{arg1})
	fake.setHeaderMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildStream) SetHeaderCallCount() int {
	fake.setHeaderMutex.RLock()
	defer fake.setHeaderMutex.RUnlock()
	return len(fake.setHeaderArgsForCall)
}

func (fake *BuildStream) SetHeaderCalls(stub func(metadata.MD) error) {
	fake.setHeaderMutex.Lock()
	defer fake.setHeaderMutex.Unlock()
	fake.SetHeaderStub = stub
}

func (fake *BuildStream) SetHeaderArgsForCall(i int) metadata.MD {
	fake.setHeaderMutex.RLock()
	defer fake.setHeaderMutex.RUnlock()
	argsForCall := fake.setHeaderArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) SetHeaderReturns(result1 error) {
	fake.setHeaderMutex.Lock()
	defer fake.setHeaderMutex.Unlock()
	fake.SetHeaderStub = nil
	fake.setHeaderReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SetHeaderReturnsOnCall(i int, result1 error) {
	fake.setHeaderMutex.Lock()
	defer fake.setHeaderMutex.Unlock()
	fake.SetHeaderStub = nil
	if fake.setHeaderReturnsOnCall == nil {
		fake.setHeaderReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.setHeaderReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildStream) SetTrailer(arg1 metadata.MD) {
	fake.setTrailerMutex.Lock()
	fake.setTrailerArgsForCall = append(fake.setTrailerArgsForCall, struct {
		arg1 metadata.MD
	}{arg1})
	stub := fake.SetTrailerStub
	fake.recordInvocation("SetTrailer", []interface{}{arg1})
	fake.setTrailerMutex.Unlock()
	if stub != nil {
		fake.SetTrailerStub(arg1)
	}
}

func (fake *BuildStream) SetTrailerCallCount() int {
	fake.setTrailerMutex.RLock()
	defer fake.setTrailerMutex.RUnlock()
	return len(fake.setTrailerArgsForCall)
}

func (fake *BuildStream) SetTrailerCalls(stub func(metadata.MD)) {
	fake.setTrailerMutex.Lock()
	defer fake.setTrailerMutex.Unlock()
	fake.SetTrailerStub = stub
}

func (fake *BuildStream) SetTrailerArgsForCall(i int) metadata.MD {
	fake.setTrailerMutex.RLock()
	defer fake.setTrailerMutex.RUnlock()
	argsForCall := fake.setTrailerArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BuildStream) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.contextMutex.RLock()
	defer fake.contextMutex.RUnlock()
	fake.recvMsgMutex.RLock()
	defer fake.recvMsgMutex.RUnlock()
	fake.sendMutex.RLock()
	defer fake.sendMutex.RUnlock()
	fake.sendHeaderMutex.RLock()
	defer fake.sendHeaderMutex.RUnlock()
	fake.sendMsgMutex.RLock()
	defer fake.sendMsgMutex.RUnlock()
	fake.setHeaderMutex.RLock()
	defer fake.setHeaderMutex.RUnlock()
	fake.setTrailerMutex.RLock()
	defer fake.setTrailerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *BuildStream) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
