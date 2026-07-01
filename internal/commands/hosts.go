package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

func newHostsCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "hosts",
		Usage: "hosthalla [--config <file>] hosts <command>",
		Short: "Manage hosts.",
		Children: []*cliapp.Command{
			newHostsListCommand(),
			newHostsShowCommand(),
			newHostsDeleteCommand(),
		},
	}
}

func newHostsListCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "list",
		Usage:       "hosthalla [--config <file>] [--json] hosts list",
		Short:       "List hosts.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runHostsList,
	}
}

func newHostsShowCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "show",
		Usage:       "hosthalla [--config <file>] [--json] hosts show <host-id>",
		Short:       "Show a host.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runHostsShow,
	}
}

func newHostsDeleteCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "delete",
		Usage:       "hosthalla [--config <file>] hosts delete <host-id>",
		Short:       "Delete a host.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runHostsDelete,
	}
}

func runHostsList(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "hosts list does not accept arguments", Usage: "hosthalla [--config <file>] [--json] hosts list"}
	}
	service, err := hostServiceFromEnv(env)
	if err != nil {
		return err
	}
	hosts, err := service.ListHosts(ctx, host.ListHostsFilter{})
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, hosts)
	}
	rows := make([][]string, 0, len(hosts))
	for _, value := range hosts {
		agentID := "-"
		if value.MonitoringAgentID != uuid.Nil {
			agentID = value.MonitoringAgentID.String()
		}
		rows = append(rows, []string{value.ID.String(), value.Name, value.IP.String(), formatList(value.Tags), agentID, formatTime(value.CreatedAt)})
	}
	printRows(env.Stdout, []string{"ID", "NAME", "IP", "TAGS", "AGENT_ID", "CREATED"}, rows)
	return nil
}

func runHostsShow(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for hosts show", Usage: "hosthalla [--config <file>] [--json] hosts show <host-id>"}
	}
	hostID, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("parse host id: %w", err)
	}
	service, err := hostServiceFromEnv(env)
	if err != nil {
		return err
	}
	value, err := service.GetHostByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("show host: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, value)
	}
	agentID := "-"
	if value.MonitoringAgentID != uuid.Nil {
		agentID = value.MonitoringAgentID.String()
	}
	fmt.Fprintf(env.Stdout, "ID: %s\nName: %s\nDescription: %s\nIP: %s\nTags: %s\nAgent ID: %s\nCreated: %s\nUpdated: %s\n", value.ID, value.Name, value.Description, value.IP, formatList(value.Tags), agentID, formatTime(value.CreatedAt), formatTime(value.UpdatedAt))
	return nil
}

func runHostsDelete(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for hosts delete", Usage: "hosthalla [--config <file>] hosts delete <host-id>"}
	}
	hostID, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("parse host id: %w", err)
	}
	service, err := hostServiceFromEnv(env)
	if err != nil {
		return err
	}
	if err := service.DeleteHost(ctx, hostID); err != nil {
		return fmt.Errorf("delete host: %w", err)
	}
	fmt.Fprintf(env.Stdout, "Host deleted: %s\n", hostID)
	return nil
}

func hostServiceFromEnv(env *cliapp.Env) (*host.Service, error) {
	secretKey, err := env.Config.SecretEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("load secret encryption key: %w", err)
	}
	service, err := newHostService(env.Logger, env.DB, secretKey)
	if err != nil {
		return nil, fmt.Errorf("create host service: %w", err)
	}
	return service, nil
}
