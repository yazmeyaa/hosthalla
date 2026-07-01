package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			*f = append(*f, item)
		}
	}
	return nil
}

func newTokensCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "tokens",
		Usage: "hosthalla [--config <file>] tokens <command>",
		Short: "Manage API tokens.",
		Children: []*cliapp.Command{
			newTokensListCommand(),
			newTokensCreateCommand(),
			newTokensShowCommand(),
			newTokensRevokeCommand(),
		},
	}
}

func newTokensListCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "list",
		Usage:       "hosthalla [--config <file>] [--json] tokens list [--user <user-id-or-username>]",
		Short:       "List API tokens.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runTokensList,
	}
}

func newTokensCreateCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "create",
		Usage:       "hosthalla [--config <file>] [--json] tokens create --user <user-id-or-username> --name <name> [--scope <scope>] [--ttl <duration>]",
		Short:       "Create an API token.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runTokensCreate,
	}
}

func newTokensShowCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "show",
		Usage:       "hosthalla [--config <file>] [--json] tokens show <token-id>",
		Short:       "Show an API token.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runTokensShow,
	}
}

func newTokensRevokeCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "revoke",
		Usage:       "hosthalla [--config <file>] tokens revoke <token-id>",
		Short:       "Revoke an API token.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runTokensRevoke,
	}
}

func runTokensList(ctx context.Context, env *cliapp.Env, args []string) error {
	flags := flag.NewFlagSet("hosthalla tokens list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	userValue := flags.String("user", "", "user id or username")
	if err := flags.Parse(args); err != nil {
		return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla [--config <file>] [--json] tokens list [--user <user-id-or-username>]"}
	}
	if flags.NArg() != 0 {
		return cliapp.UsageError{Message: "tokens list does not accept positional arguments", Usage: "hosthalla [--config <file>] [--json] tokens list [--user <user-id-or-username>]"}
	}

	service := newAuthenticationService(env.DB)
	var tokens []authentication.APIToken
	var err error
	if strings.TrimSpace(*userValue) == "" {
		tokens, err = service.ListAPITokens(ctx)
	} else {
		user, resolveErr := resolveUser(ctx, service, *userValue)
		if resolveErr != nil {
			return fmt.Errorf("resolve user: %w", resolveErr)
		}
		tokens, err = service.ListAPITokensByProfileID(ctx, user.ID)
	}
	if err != nil {
		return fmt.Errorf("list API tokens: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, tokens)
	}

	rows := make([][]string, 0, len(tokens))
	for _, token := range tokens {
		rows = append(rows, []string{token.ID, token.ProfileID, token.Name, formatList(token.Scopes), formatOptionalTime(token.ExpiresAt), formatOptionalTime(token.RevokedAt)})
	}
	printRows(env.Stdout, []string{"ID", "USER_ID", "NAME", "SCOPES", "EXPIRES", "REVOKED"}, rows)
	return nil
}

func runTokensCreate(ctx context.Context, env *cliapp.Env, args []string) error {
	flags := flag.NewFlagSet("hosthalla tokens create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	userValue := flags.String("user", "", "user id or username")
	name := flags.String("name", "", "token name")
	ttl := flags.Duration("ttl", 0, "token time-to-live")
	var scopes stringListFlag
	flags.Var(&scopes, "scope", "token scope, repeatable or comma-separated")
	if err := flags.Parse(args); err != nil {
		return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla [--config <file>] [--json] tokens create --user <user-id-or-username> --name <name> [--scope <scope>] [--ttl <duration>]"}
	}
	if flags.NArg() != 0 {
		return cliapp.UsageError{Message: "tokens create does not accept positional arguments", Usage: "hosthalla [--config <file>] [--json] tokens create --user <user-id-or-username> --name <name> [--scope <scope>] [--ttl <duration>]"}
	}

	service := newAuthenticationService(env.DB)
	user, err := resolveUser(ctx, service, *userValue)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	result, err := service.CreateAPIToken(ctx, auth_service.CreateAPITokenDTO{
		ProfileID: user.ID,
		Name:      *name,
		Scopes:    scopes,
		ExpiresIn: time.Duration(*ttl),
	})
	if err != nil {
		return fmt.Errorf("create API token: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, result)
	}
	fmt.Fprintf(env.Stdout, "Token created: %s\nPlain token: %s\n", result.Token.ID, result.PlainToken)
	return nil
}

func runTokensShow(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for tokens show", Usage: "hosthalla [--config <file>] [--json] tokens show <token-id>"}
	}
	token, err := newAuthenticationService(env.DB).GetAPITokenByID(ctx, args[0])
	if err != nil {
		return fmt.Errorf("show API token: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, token)
	}
	fmt.Fprintf(env.Stdout, "ID: %s\nUser ID: %s\nName: %s\nScopes: %s\nCreated: %s\nExpires: %s\nRevoked: %s\n", token.ID, token.ProfileID, token.Name, formatList(token.Scopes), formatTime(token.CreatedAt), formatOptionalTime(token.ExpiresAt), formatOptionalTime(token.RevokedAt))
	return nil
}

func runTokensRevoke(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for tokens revoke", Usage: "hosthalla [--config <file>] tokens revoke <token-id>"}
	}
	if err := newAuthenticationService(env.DB).RevokeAPIToken(ctx, args[0]); err != nil {
		return fmt.Errorf("revoke API token: %w", err)
	}
	fmt.Fprintf(env.Stdout, "Token revoked: %s\n", args[0])
	return nil
}
