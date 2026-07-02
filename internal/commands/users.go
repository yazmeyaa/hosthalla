package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

type userCreator interface {
	CreateUser(ctx context.Context, data auth_service.CreateUserDTO) (authentication.Profile, error)
}

var newUserCreator = func(pool *pgxpool.Pool) userCreator {
	return newAuthenticationService(pool)
}

func newUsersCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "users",
		Usage: "hosthalla [--config <file>] users <command>",
		Short: "Manage users.",
		Children: []*cliapp.Command{
			newUsersCreateCommand("hosthalla [--config <file>] users create <username> <password>"),
			newUsersListCommand(),
			newUsersShowCommand(),
			newUsersDeleteCommand(),
			{
				Name:  "password",
				Usage: "hosthalla [--config <file>] users password <command>",
				Short: "Manage user passwords.",
				Children: []*cliapp.Command{
					newUsersPasswordSetCommand(),
				},
			},
		},
	}
}

func newUsersCreateCommand(usage string) *cliapp.Command {
	return &cliapp.Command{
		Name:        "create",
		Usage:       usage,
		Short:       "Create a user.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runUsersCreate,
	}
}

func newUsersListCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "list",
		Usage:       "hosthalla [--config <file>] [--json] users list",
		Short:       "List users.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runUsersList,
	}
}

func newUsersShowCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "show",
		Usage:       "hosthalla [--config <file>] [--json] users show <user-id-or-username>",
		Short:       "Show a user.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runUsersShow,
	}
}

func newUsersDeleteCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "delete",
		Usage:       "hosthalla [--config <file>] users delete <user-id-or-username>",
		Short:       "Delete a user.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runUsersDelete,
	}
}

func newUsersPasswordSetCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "set",
		Usage:       "hosthalla [--config <file>] users password set <user-id-or-username> <password>",
		Short:       "Set a user password.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runUsersPasswordSet,
	}
}

func runUsersCreate(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 2 {
		return cliapp.UsageError{
			Message: "invalid arguments for users create",
			Usage:   "hosthalla [--config <file>] users create <username> <password>",
		}
	}

	user, err := newUserCreator(env.DB).CreateUser(ctx, auth_service.CreateUserDTO{
		Username: args[0],
		Password: args[1],
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Fprintf(env.Stdout, "User created: %s (%s)\n", user.Username, user.ID)
	if env.Logger != nil {
		env.Logger.Info("user created",
			"user_id", user.ID,
			"username", user.Username,
			"created_at", user.CreatedAt.Format(time.RFC3339),
			"updated_at", user.UpdatedAt.Format(time.RFC3339),
		)
	}
	return nil
}

func runUsersList(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "users list does not accept arguments", Usage: "hosthalla [--config <file>] [--json] users list"}
	}

	users, err := newAuthenticationService(env.DB).ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, users)
	}

	rows := make([][]string, 0, len(users))
	for _, user := range users {
		rows = append(rows, []string{user.ID, user.Username, formatTime(user.CreatedAt)})
	}
	printRows(env.Stdout, []string{"ID", "USERNAME", "CREATED"}, rows)
	return nil
}

func runUsersShow(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for users show", Usage: "hosthalla [--config <file>] [--json] users show <user-id-or-username>"}
	}

	user, err := resolveUser(ctx, newAuthenticationService(env.DB), args[0])
	if err != nil {
		return fmt.Errorf("show user: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, user)
	}
	fmt.Fprintf(env.Stdout, "ID: %s\nUsername: %s\nCreated: %s\nUpdated: %s\n", user.ID, user.Username, formatTime(user.CreatedAt), formatTime(user.UpdatedAt))
	return nil
}

func runUsersDelete(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for users delete", Usage: "hosthalla [--config <file>] users delete <user-id-or-username>"}
	}

	service := newAuthenticationService(env.DB)
	user, err := resolveUser(ctx, service, args[0])
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if err := service.DeleteUser(ctx, user.ID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	fmt.Fprintf(env.Stdout, "User deleted: %s (%s)\n", user.Username, user.ID)
	return nil
}

func runUsersPasswordSet(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 2 {
		return cliapp.UsageError{Message: "invalid arguments for users password set", Usage: "hosthalla [--config <file>] users password set <user-id-or-username> <password>"}
	}

	service := newAuthenticationService(env.DB)
	user, err := resolveUser(ctx, service, args[0])
	if err != nil {
		return fmt.Errorf("set user password: %w", err)
	}
	if _, err := service.SetPassword(ctx, auth_service.SetPasswordDTO{ProfileID: user.ID, Password: args[1]}); err != nil {
		return fmt.Errorf("set user password: %w", err)
	}
	fmt.Fprintf(env.Stdout, "Password updated for user: %s (%s)\n", user.Username, user.ID)
	return nil
}

func resolveUser(ctx context.Context, service *auth_service.Service, value string) (authentication.Profile, error) {
	identifier := strings.TrimSpace(value)
	if identifier == "" {
		return authentication.Profile{}, fmt.Errorf("user id or username is required")
	}
	if user, err := service.GetProfileByID(ctx, identifier); err == nil {
		return user, nil
	}
	return service.GetProfileByUsername(ctx, identifier)
}
