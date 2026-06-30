package cli

import (
	"context"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/config"
)

type RunFunc func(ctx context.Context, env *Env, args []string) error

type Command struct {
	Name        string
	Aliases     []string
	Usage       string
	Short       string
	Children    []*Command
	Run         RunFunc
	NeedsConfig bool
	NeedsDB     bool
}

type Env struct {
	Stdout     io.Writer
	Stderr     io.Writer
	ConfigPath string
	JSON       bool
	Config     *config.AppConfig
	DB         *pgxpool.Pool
	Logger     *slog.Logger
}

func (c *Command) matches(name string) bool {
	if c == nil {
		return false
	}
	if c.Name == name {
		return true
	}
	for _, alias := range c.Aliases {
		if alias == name {
			return true
		}
	}
	return false
}

func (c *Command) child(name string) *Command {
	for _, child := range c.Children {
		if child.matches(name) {
			return child
		}
	}
	return nil
}
