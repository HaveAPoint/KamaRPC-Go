package server

import (
	"context"
	"testing"
)

func TestHandler_InvokeUnknownService(t *testing.T) {
	h := mustNewHandler()

	_, err := h.invoke(
		context.Background(),
		nil,
		"MissingService",
		"Add",
		nil,
	)
	if err == nil {
		t.Fatal("invoke() error = nil, want unknown service error")
	}
}
