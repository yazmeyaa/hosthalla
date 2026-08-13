package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yazmeyaa/hosthalla/internal/host"
)

type metricsCollectorFunc func(context.Context) (host.HostMetric, error)

func (f metricsCollectorFunc) GetMetrics(ctx context.Context) (host.HostMetric, error) { return f(ctx) }

func TestCachedMetricsCollectorCoalescesConcurrentCalls(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	collector := NewCachedMetricsCollector(metricsCollectorFunc(func(context.Context) (host.HostMetric, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		close(started)
		<-release
		return host.HostMetric{MemoryUsageBytes: 42}, nil
	}), time.Minute)

	const workers = 8
	results := make(chan error, workers)
	for range workers {
		go func() {
			metric, err := collector.GetMetrics(context.Background())
			if err == nil && metric.MemoryUsageBytes != 42 {
				err = errors.New("unexpected metric")
			}
			results <- err
		}()
	}
	<-started
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("backend calls = %d, want 1", gotCalls)
	}
	close(release)
	for range workers {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestCachedMetricsCollectorCachesErrors(t *testing.T) {
	wantErr := errors.New("collection failed")
	calls := 0
	collector := NewCachedMetricsCollector(metricsCollectorFunc(func(context.Context) (host.HostMetric, error) {
		calls++
		return host.HostMetric{}, wantErr
	}), time.Minute)

	for range 2 {
		_, err := collector.GetMetrics(context.Background())
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	}
	if calls != 1 {
		t.Fatalf("backend calls = %d, want 1", calls)
	}
}

func TestCachedMetricsCollectorRefreshesAfterTTL(t *testing.T) {
	calls := 0
	collector := NewCachedMetricsCollector(metricsCollectorFunc(func(context.Context) (host.HostMetric, error) {
		calls++
		return host.HostMetric{MemoryUsageBytes: uint64(calls)}, nil
	}), time.Millisecond)

	first, err := collector.GetMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := collector.GetMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.MemoryUsageBytes != 1 || second.MemoryUsageBytes != 2 {
		t.Fatalf("metrics = %d, %d; want 1, 2", first.MemoryUsageBytes, second.MemoryUsageBytes)
	}
}

func TestCachedMetricsCollectorWaitingContextCanCancel(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	collector := NewCachedMetricsCollector(metricsCollectorFunc(func(context.Context) (host.HostMetric, error) {
		close(started)
		<-release
		return host.HostMetric{}, nil
	}), time.Minute)
	ownerDone := make(chan struct{})
	go func() {
		_, _ = collector.GetMetrics(context.Background())
		close(ownerDone)
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := collector.GetMetrics(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	close(release)
	<-ownerDone
}

func TestCachedMetricsCollectorPanicDoesNotBlockWaiters(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	collector := NewCachedMetricsCollector(metricsCollectorFunc(func(context.Context) (host.HostMetric, error) {
		close(started)
		<-release
		panic("boom")
	}), time.Minute)
	ownerDone := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(ownerDone)
		}()
		_, _ = collector.GetMetrics(context.Background())
	}()
	<-started

	waiterDone := make(chan error, 1)
	go func() {
		_, err := collector.GetMetrics(context.Background())
		waiterDone <- err
	}()
	close(release)
	<-ownerDone

	select {
	case err := <-waiterDone:
		if err == nil || !strings.Contains(err.Error(), "metrics collector panic") {
			t.Fatalf("waiter error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter remained blocked after collector panic")
	}
}
