package llm

import (
	"context"
	"testing"
)

func TestCompletionReadyForCache(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		ctx    context.Context
		reason string
		text   string
		want   bool
	}{
		{name: "stop", ctx: ctx, reason: "stop", text: "answer", want: true},
		{name: "length", ctx: ctx, reason: "length", text: "partial", want: true},
		{name: "empty reason", ctx: ctx, reason: "", text: "partial", want: false},
		{name: "empty text", ctx: ctx, reason: "stop", text: "  ", want: false},
		{name: "cancelled ctx", ctx: canceledContext(), reason: "stop", text: "answer", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompletionReadyForCache(tc.ctx, tc.reason, tc.text); got != tc.want {
				t.Fatalf("CompletionReadyForCache() = %v, want %v", got, tc.want)
			}
		})
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
