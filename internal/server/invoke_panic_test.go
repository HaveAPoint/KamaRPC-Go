package server

import (
	"context"
	"strings"
	"testing"
)

type panicArgs struct {
	A int
}

type panicReply struct {
	Sum int
}

type panicService struct{}

// Boom 模拟业务代码里的空指针解引用
func (p *panicService) Boom(args *panicArgs, reply *panicReply) error {
	var nilPtr *panicReply
	reply.Sum = nilPtr.Sum // 必然 panic
	return nil
}

func TestHandler_InvokePanicBecomesError(t *testing.T) {
	h := mustNewHandler()

	result, err := h.invoke(
		context.Background(),
		&panicService{},
		"PanicService",
		"Boom",
		[]byte(`{"A":1}`),
	)

	if err == nil {
		t.Fatal("invoke() error = nil, want panic converted to error")
	}
	if result != nil {
		t.Fatalf("invoke() result = %v, want nil", result)
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("invoke() error = %q, want it to mention panic", err)
	}
	// 关键断言：进程还活着，测试能跑到这里就说明 recover 生效了
}
