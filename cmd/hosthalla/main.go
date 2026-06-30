package main

import (
	"context"
	"os"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/commands"
)

func main() {
	root := commands.NewRoot(commands.RootParams{})
	code := cliapp.Execute(context.Background(), root, os.Args[1:], os.Stdout, os.Stderr, cliapp.DefaultDependencies())
	os.Exit(code)
}
