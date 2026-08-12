package commands

import (
	"log/slog"

	agent_domain "github.com/yazmeyaa/hosthalla/internal/agent"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	appdatabase "github.com/yazmeyaa/hosthalla/internal/database"
	"github.com/yazmeyaa/hosthalla/internal/events"
	host_domain "github.com/yazmeyaa/hosthalla/internal/host"
)

func newAuthenticationService(store *appdatabase.Store) *auth_service.Service {
	return auth_service.New(auth_service.NewParams{
		ProfileRepository:                store.ProfileRepository,
		PasswordAuthenticationRepository: store.PasswordAuthenticationRepository,
		SessionRepository:                store.SessionRepository,
		APITokenRepository:               store.APITokenRepository,
	})
}

func newHostService(logger *slog.Logger, store *appdatabase.Store, secretKey []byte) (*host_domain.Service, error) {
	secretCipher, err := host_domain.NewAESGCMSecretCipher(secretKey)
	if err != nil {
		return nil, err
	}
	return host_domain.NewService(host_domain.NewServiceParams{
		HostRepository:                 store.HostRepository,
		HostManagementMethodRepository: store.HostManagementMethodRepository,
		HostSystemInfoRepository:       store.HostSystemInfoRepository,
		HostMetricSnapshotRepository:   store.HostMetricSnapshotRepository,
		SecretCipher:                   secretCipher,
		Logger:                         logger,
		EventBus:                       events.NewInMemoryEventBus(),
	}), nil
}

func newAgentAdminService(logger *slog.Logger, store *appdatabase.Store) *agent_domain.Service {
	return agent_domain.NewService(agent_domain.NewServiceParams{
		AgentRepository:       store.AgentRepository,
		AgentConfigRepository: store.AgentConfigRepository,
		Logger:                logger,
		EventBus:              events.NewInMemoryEventBus(),
	})
}
