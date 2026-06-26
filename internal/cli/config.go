package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yazmeyaa/hosthalla/internal/config"
)

func ProcessConfigCommand(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}

	defaultPath := config.DefaultConfigPath
	commandArgs := args
	if strings.HasPrefix(args[0], "-") {
		globalFlags := flag.NewFlagSet("hosthalla config", flag.ContinueOnError)
		globalFlags.SetOutput(io.Discard)
		path := globalFlags.String("path", config.DefaultConfigPath, "path to config file")

		if err := globalFlags.Parse(args); err != nil {
			fmt.Printf("Failed to parse flags: %s\n", err)
			printConfigUsage()
			os.Exit(1)
		}

		defaultPath = *path
		commandArgs = globalFlags.Args()
	}

	if len(commandArgs) == 0 {
		printConfigUsage()
		os.Exit(1)
	}

	command := commandArgs[0]
	subCommandArgs := commandArgs[1:]

	switch command {
	case "generate":
		processConfigGenerateCommand(subCommandArgs, defaultPath)
	case "show":
		processConfigShowCommand(subCommandArgs, defaultPath)
	default:
		fmt.Printf("Unknown config command %q\n", command)
		printConfigUsage()
		os.Exit(1)
	}
}

func processConfigGenerateCommand(args []string, defaultPath string) {
	flags := flag.NewFlagSet("hosthalla config generate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	path := flags.String("path", defaultPath, "path to config file")
	overwrite := flags.Bool("overwrite", false, "overwrite existing config file")

	if err := flags.Parse(args); err != nil {
		fmt.Printf("Failed to parse flags: %s\n", err)
		printGenerateUsage()
		os.Exit(1)
	}

	if flags.NArg() != 0 {
		printGenerateUsage()
		os.Exit(1)
	}

	if err := config.GenerateDefaultConfig(*path, *overwrite); err != nil {
		if errors.Is(err, config.ErrConfigAlreadyExists) {
			fmt.Printf("Config already exists at %q. Use --overwrite to replace it.\n", *path)
			os.Exit(1)
		}

		fmt.Printf("Failed to generate config: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config generated at %q\n", *path)
}

func processConfigShowCommand(args []string, defaultPath string) {
	flags := flag.NewFlagSet("hosthalla config show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	path := flags.String("path", defaultPath, "path to config file")
	if err := flags.Parse(args); err != nil {
		fmt.Printf("Failed to parse flags: %s\n", err)
		printShowUsage()
		os.Exit(1)
	}

	if flags.NArg() != 0 {
		printShowUsage()
		os.Exit(1)
	}

	content, err := config.ReadYAMLFromPath(*path)
	if err != nil {
		fmt.Printf("Failed to read config: %s\n", err)
		os.Exit(1)
	}

	fmt.Print(string(content))
}

func printConfigUsage() {
	fmt.Println("Usage:")
	fmt.Println("  hosthalla config generate [--path <file>] [--overwrite]")
	fmt.Println("  hosthalla config show [--path <file>]")
}

func printGenerateUsage() {
	fmt.Println("Usage: hosthalla config generate [--path <file>] [--overwrite]")
}

func printShowUsage() {
	fmt.Println("Usage: hosthalla config show [--path <file>]")
}
