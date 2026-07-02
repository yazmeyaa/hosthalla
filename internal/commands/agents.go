package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	agent_domain "github.com/yazmeyaa/hosthalla/internal/agent"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

func newAgentsCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "agents",
		Usage: "hosthalla [--config <file>] agents <command>",
		Short: "Manage registered agents.",
		Children: []*cliapp.Command{
			newAgentsListCommand(),
			newAgentsShowCommand(),
			newAgentsDeleteCommand(),
		},
	}
}

func newAgentsListCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "list",
		Usage:       "hosthalla [--config <file>] [--json] agents list",
		Short:       "List registered agents.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runAgentsList,
	}
}

func newAgentsShowCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "show",
		Usage:       "hosthalla [--config <file>] [--json] agents show <agent-id>",
		Short:       "Show a registered agent.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runAgentsShow,
	}
}

func newAgentsDeleteCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:        "delete",
		Usage:       "hosthalla [--config <file>] agents delete <agent-id>",
		Short:       "Delete a registered agent.",
		NeedsConfig: true,
		NeedsDB:     true,
		Run:         runAgentsDelete,
	}
}

func runAgentsList(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "agents list does not accept arguments", Usage: "hosthalla [--config <file>] [--json] agents list"}
	}
	agents, err := newAgentAdminService(env.Logger, env.DB).ListAgents(ctx)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, agents)
	}
	rows := make([][]string, 0, len(agents))
	for _, value := range agents {
		rows = append(rows, agentRow(value))
	}
	printRows(env.Stdout, []string{"ID", "HOST_ID", "VERSION", "LAST_SEEN", "CREATED"}, rows)
	return nil
}

func runAgentsShow(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for agents show", Usage: "hosthalla [--config <file>] [--json] agents show <agent-id>"}
	}
	agentID, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("parse agent id: %w", err)
	}
	value, err := newAgentAdminService(env.Logger, env.DB).GetByID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("show agent: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, value)
	}
	fmt.Fprintf(env.Stdout, "ID: %s\nHost ID: %s\nVersion: %s\nLast seen: %s\nCreated: %s\n", value.ID, value.HostID, value.Version, formatTime(value.LastSeenAt), formatTime(value.CreatedAt))
	return nil
}

func runAgentsDelete(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 1 {
		return cliapp.UsageError{Message: "invalid arguments for agents delete", Usage: "hosthalla [--config <file>] agents delete <agent-id>"}
	}
	agentID, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("parse agent id: %w", err)
	}
	if err := newAgentAdminService(env.Logger, env.DB).DeleteAgent(ctx, agentID); err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	fmt.Fprintf(env.Stdout, "Agent deleted: %s\n", agentID)
	return nil
}

func agentRow(value agent_domain.Agent) []string {
	return []string{value.ID.String(), value.HostID.String(), value.Version, formatTime(value.LastSeenAt), formatTime(value.CreatedAt)}
}
