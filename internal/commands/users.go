package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	auth_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

type userCreator interface {
	CreateUser(ctx context.Context, data auth_service.CreateUserDTO) (authentication.Profile, error)
}

var newUserCreator = func(pool *pgxpool.Pool) userCreator {
	return auth_service.New(auth_service.NewParams{
		ProfileRepository:                auth_storage.NewProfileRepository(pool),
		PasswordAuthenticationRepository: auth_storage.NewPasswordAuthenticationRepository(pool),
		SessionRepository:                auth_storage.NewSessionRepository(pool),
		APITokenRepository:               auth_storage.NewAPITokenRepository(pool),
	})
}

func newUsersCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "users",
		Usage: "hosthalla [--config <file>] users <command>",
		Short: "Manage users.",
		Children: []*cliapp.Command{
			newUsersCreateCommand("hosthalla [--config <file>] users create <username> <password>"),
			newUsersPlaceholderCommand("list", "List users."),
			newUsersPlaceholderCommand("show", "Show a user."),
			newUsersPlaceholderCommand("delete", "Delete a user."),
			{
				Name:  "password",
				Usage: "hosthalla [--config <file>] users password <command>",
				Short: "Manage user passwords.",
				Children: []*cliapp.Command{
					newUsersPlaceholderCommand("set", "Set a user password."),
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

func newUsersPlaceholderCommand(name string, short string) *cliapp.Command {
	return &cliapp.Command{
		Name:  name,
		Usage: "hosthalla [--config <file>] users " + name,
		Short: short,
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			return fmt.Errorf("users %s is not implemented yet", name)
		},
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
