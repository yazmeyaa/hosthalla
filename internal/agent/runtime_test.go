package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeKeepsOtherInstancesRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var healthyRan atomic.Bool
	runtime := NewRuntime([]Instance{
		{Name: "broken", Run: func(context.Context) error { panic("boom") }},
		{Name: "healthy", Run: func(ctx context.Context) error {
			healthyRan.Store(true)
			<-ctx.Done()
			return nil
		}},
	}, discardLogger())

	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()

	deadline := time.After(time.Second)
	for !healthyRan.Load() {
		select {
		case err := <-done:
			t.Fatalf("runtime stopped early: %v", err)
		case <-deadline:
			t.Fatal("healthy instance did not start")
		default:
		}
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error after cancellation = %v", err)
	}
}

func TestRuntimeReturnsErrorWhenAllInstancesStop(t *testing.T) {
	runtime := NewRuntime([]Instance{
		{Name: "one", Run: func(context.Context) error { return nil }},
		{Name: "two", Run: func(context.Context) error { return nil }},
	}, discardLogger())

	err := runtime.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "all agent instances stopped") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRuntimeWaitsForGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	runtime := NewRuntime([]Instance{{
		Name: "slow",
		Run: func(ctx context.Context) error {
			<-ctx.Done()
			<-release
			return nil
		},
	}}, discardLogger())

	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	cancel()

	select {
	case <-done:
		t.Fatal("runtime did not wait for instance shutdown")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
