package cli

import (
	"errors"
	"fmt"
)

const (
	ExitCodeOK    = 0
	ExitCodeError = 1
	ExitCodeUsage = 2
)

type UsageError struct {
	Message string
	Usage   string
}

func (e UsageError) Error() string {
	if e.Message == "" {
		return "invalid usage"
	}
	return e.Message
}

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit with code %d", e.Code)
	}
	return e.Err.Error()
}

func (e ExitError) Unwrap() error {
	return e.Err
}

func ExitCode(err error) int {
	if err == nil {
		return ExitCodeOK
	}

	var exitErr ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Code == 0 {
			return ExitCodeOK
		}
		return exitErr.Code
	}

	var usageErr UsageError
	if errors.As(err, &usageErr) {
		return ExitCodeUsage
	}

	return ExitCodeError
}
