package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	agentsqlite "github.com/yazmeyaa/hosthalla/internal/agent/sqlite"
	"github.com/yazmeyaa/hosthalla/internal/host"
	hostsqlite "github.com/yazmeyaa/hosthalla/internal/host/sqlite"
	"github.com/yazmeyaa/hosthalla/internal/testsqlite"
)

func TestAgentRepositories(t *testing.T) {
	ctx := context.Background()
	db := testsqlite.Open(t)
	hosts := hostsqlite.NewHostRepository(db)
	agents := agentsqlite.NewAgentRepository(db)
	configs := agentsqlite.NewAgentConfigRepository(db)

	hostOne, err := hosts.CreateHost(ctx, host.CreateHostDTO{Name: "one", IP: netip.MustParseAddr("192.0.2.1")})
	must(t, err)
	hostTwo, err := hosts.CreateHost(ctx, host.CreateHostDTO{Name: "two", IP: netip.MustParseAddr("192.0.2.2")})
	must(t, err)

	value, err := agents.Create(ctx, agent.CreateAgentDTO{HostID: hostOne.ID, Version: "1.0.0"})
	must(t, err)
	byID, err := agents.GetByID(ctx, value.ID)
	must(t, err)
	byHost, err := agents.GetByHostID(ctx, hostOne.ID)
	must(t, err)
	if byID.ID != value.ID || byHost.ID != value.ID {
		t.Fatal("agent lookup returned another agent")
	}
	listed, err := agents.List(ctx)
	must(t, err)
	if len(listed) != 1 {
		t.Fatalf("agents count = %d", len(listed))
	}
	if _, err := agents.Create(ctx, agent.CreateAgentDTO{HostID: hostOne.ID, Version: "duplicate"}); err == nil {
		t.Fatal("expected one-agent-per-host constraint error")
	}

	value.Version = "1.1.0"
	must(t, agents.Update(ctx, &value))
	lastSeenAt := time.Now().UTC().Add(time.Minute)
	must(t, agents.UpdateLastSeenAt(ctx, value.ID, lastSeenAt))
	updated, err := agents.GetByID(ctx, value.ID)
	must(t, err)
	if updated.Version != "1.1.0" || !updated.LastSeenAt.Equal(lastSeenAt) {
		t.Fatalf("agent was not updated: %+v", updated)
	}

	defaultConfig, err := configs.GetByAgentID(ctx, value.ID)
	must(t, err)
	if defaultConfig.Heartbeat.Interval != agent.DefaultAgentHeartbeatInterval || defaultConfig.Metrics.Interval != agent.DefaultAgentMetricsInterval || defaultConfig.Version != 1 {
		t.Fatalf("unexpected default config: %+v", defaultConfig)
	}
	defaultConfig.Heartbeat.Interval = 9 * time.Second
	defaultConfig.Metrics.Interval = 11 * time.Second
	defaultConfig.Version = 2
	must(t, configs.Update(ctx, &defaultConfig))
	updatedConfig, err := configs.GetByAgentID(ctx, value.ID)
	must(t, err)
	if updatedConfig.Heartbeat.Interval != 9*time.Second || updatedConfig.Metrics.Interval != 11*time.Second || updatedConfig.Version != 2 {
		t.Fatalf("config was not updated: %+v", updatedConfig)
	}

	second, err := agents.Create(ctx, agent.CreateAgentDTO{HostID: hostTwo.ID, Version: "2.0.0"})
	must(t, err)
	explicitConfig, err := configs.Create(ctx, agent.CreateAgentConfigDTO{
		AgentID: second.ID, Heartbeat: agent.AgentHeartbeatConfig{Interval: 3 * time.Second},
		Metrics: agent.AgentMetricsConfig{Interval: 7 * time.Second}, Version: 4,
	})
	must(t, err)
	if explicitConfig.Version != 4 {
		t.Fatalf("explicit config = %+v", explicitConfig)
	}
	if _, err := configs.Create(ctx, agent.CreateAgentConfigDTO{AgentID: uuid.New()}); err == nil {
		t.Fatal("expected agent config foreign key error")
	}

	must(t, agents.Delete(ctx, value.ID))
	if _, err := agents.GetByID(ctx, value.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted agent error = %v", err)
	}
	var configCount int
	must(t, db.QueryRowContext(ctx, `select count(*) from agent_config where agent_id = ?`, value.ID.String()).Scan(&configCount))
	if configCount != 0 {
		t.Fatalf("agent config cascade count = %d", configCount)
	}
	if err := agents.Delete(ctx, value.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second agent delete error = %v", err)
	}
	if err := configs.Update(ctx, &agent.AgentConfig{ID: uuid.New()}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing config update error = %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
