package commands

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	agent_domain "github.com/yazmeyaa/hosthalla/internal/agent"
	agent_storage "github.com/yazmeyaa/hosthalla/internal/agent/postgres"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	auth_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/events"
	host_domain "github.com/yazmeyaa/hosthalla/internal/host"
	host_storage "github.com/yazmeyaa/hosthalla/internal/host/postgres"
)

func newAuthenticationService(pool *pgxpool.Pool) *auth_service.Service {
	return auth_service.New(auth_service.NewParams{
		ProfileRepository:                auth_storage.NewProfileRepository(pool),
		PasswordAuthenticationRepository: auth_storage.NewPasswordAuthenticationRepository(pool),
		SessionRepository:                auth_storage.NewSessionRepository(pool),
		APITokenRepository:               auth_storage.NewAPITokenRepository(pool),
	})
}

func newHostService(logger *slog.Logger, pool *pgxpool.Pool, secretKey []byte) (*host_domain.Service, error) {
	repositories := host_storage.NewRepositories(pool)
	secretCipher, err := host_domain.NewAESGCMSecretCipher(secretKey)
	if err != nil {
		return nil, err
	}
	return host_domain.NewService(host_domain.NewServiceParams{
		HostRepository:                 repositories.Host,
		HostManagementMethodRepository: repositories.HostManagementMethod,
		HostSystemInfoRepository:       repositories.HostSystemInfo,
		HostMetricSnapshotRepository:   repositories.HostMetricSnapshot,
		SecretCipher:                   secretCipher,
		Logger:                         logger,
		EventBus:                       events.NewInMemoryEventBus(),
	}), nil
}

func newAgentAdminService(logger *slog.Logger, pool *pgxpool.Pool) *agent_domain.Service {
	return agent_domain.NewService(agent_domain.NewServiceParams{
		AgentRepository:       agent_storage.NewAgentRepository(pool),
		AgentConfigRepository: agent_storage.NewAgentConfigRepository(pool),
		Logger:                logger,
		EventBus:              events.NewInMemoryEventBus(),
	})
}
