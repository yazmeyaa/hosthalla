package cli

import (
	"fmt"
	"strings"
)

type resolveResult struct {
	Command *Command
	Args    []string
	Help    bool
}

func resolve(root *Command, args []string) (resolveResult, error) {
	if root == nil {
		return resolveResult{}, fmt.Errorf("root command is nil")
	}
	if len(args) == 0 {
		return resolveResult{Command: root, Help: true}, nil
	}

	if args[0] == "help" {
		if len(args) == 1 {
			return resolveResult{Command: root, Help: true}, nil
		}
		cmd, rest, err := walk(root, args[1:])
		if err != nil {
			return resolveResult{}, err
		}
		if len(rest) > 0 {
			return resolveResult{}, UsageError{
				Message: fmt.Sprintf("unknown help topic %q", strings.Join(args[1:], " ")),
				Usage:   root.Usage,
			}
		}
		return resolveResult{Command: cmd, Help: true}, nil
	}

	cmd, rest, err := walk(root, args)
	if err != nil {
		return resolveResult{}, err
	}
	if len(rest) > 0 && rest[0] == "help" {
		if len(rest) > 1 {
			return resolveResult{}, UsageError{
				Message: fmt.Sprintf("unexpected arguments after help: %s", strings.Join(rest[1:], " ")),
				Usage:   cmd.Usage,
			}
		}
		return resolveResult{Command: cmd, Help: true}, nil
	}
	if hasHelpFlag(rest) {
		return resolveResult{Command: cmd, Help: true}, nil
	}
	if cmd.Run == nil {
		return resolveResult{}, UsageError{
			Message: fmt.Sprintf("missing command under %q", commandPath(cmd)),
			Usage:   cmd.Usage,
		}
	}
	return resolveResult{Command: cmd, Args: rest}, nil
}

func walk(root *Command, args []string) (*Command, []string, error) {
	cmd := root
	for idx, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return cmd, args[idx:], nil
		}
		if arg == "help" {
			return cmd, args[idx:], nil
		}

		child := cmd.child(arg)
		if child == nil {
			if cmd == root || len(cmd.Children) > 0 {
				return nil, nil, UsageError{
					Message: fmt.Sprintf("unknown command %q", arg),
					Usage:   cmd.Usage,
				}
			}
			return cmd, args[idx:], nil
		}
		cmd = child
	}
	return cmd, nil, nil
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func commandPath(cmd *Command) string {
	if cmd == nil || cmd.Name == "" {
		return ""
	}
	return cmd.Name
}
