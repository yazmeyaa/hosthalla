package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

type Instance struct {
	Name   string
	Run    func(context.Context) error
	Logger *slog.Logger
}

type Runtime struct {
	instances []Instance
	logger    *slog.Logger
}

func NewRuntime(instances []Instance, logger *slog.Logger) *Runtime {
	return &Runtime{instances: instances, logger: logger}
}

type instanceResult struct {
	name   string
	logger *slog.Logger
	err    error
	stack  string
}

func (r *Runtime) Run(ctx context.Context) error {
	if len(r.instances) == 0 {
		return fmt.Errorf("agent runtime requires at least one instance")
	}

	results := make(chan instanceResult, len(r.instances))
	for _, instance := range r.instances {
		go runInstance(ctx, instance, results)
	}

	remaining := len(r.instances)
	stopping := false
	for remaining > 0 {
		if stopping {
			result := <-results
			remaining--
			logInstanceResult(result, r.logger)
			continue
		}

		select {
		case <-ctx.Done():
			stopping = true
		case result := <-results:
			remaining--
			logInstanceResult(result, r.logger)
		}
	}

	if ctx.Err() != nil {
		return nil
	}
	return fmt.Errorf("all agent instances stopped")
}

func runInstance(ctx context.Context, instance Instance, results chan<- instanceResult) {
	result := instanceResult{name: instance.Name, logger: instance.Logger}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.err = fmt.Errorf("panic: %v", recovered)
			result.stack = string(debug.Stack())
		}
		results <- result
	}()

	result.err = instance.Run(ctx)
}

func logInstanceResult(result instanceResult, fallback *slog.Logger) {
	logger := result.logger
	if logger == nil && fallback != nil {
		logger = fallback.With("config", result.name)
	}
	if logger == nil {
		return
	}
	if result.err != nil {
		if result.stack != "" {
			logger.Error("agent instance stopped after panic", "error", result.err, "stack", result.stack)
			return
		}
		logger.Error("agent instance stopped", "error", result.err)
		return
	}
	logger.Info("agent instance stopped")
}
