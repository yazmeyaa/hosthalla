package host

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host/sshcheck"
)

type testHostRepository map[uuid.UUID]Host

func (r testHostRepository) ListHosts(context.Context, ListHostsFilter) ([]Host, error) {
	return nil, nil
}

func (r testHostRepository) ListTags(context.Context) ([]Tag, error) {
	return nil, nil
}

func (r testHostRepository) GetHostByID(_ context.Context, hostID uuid.UUID) (Host, error) {
	target, ok := r[hostID]
	if !ok {
		return Host{}, errors.New("host not found")
	}
	return target, nil
}

func (r testHostRepository) DeleteHost(context.Context, uuid.UUID) error {
	return nil
}

func (r testHostRepository) UpdateHost(context.Context, *Host) error {
	return nil
}

func (r testHostRepository) CreateHost(context.Context, CreateHostDTO) (Host, error) {
	return Host{}, nil
}

type testHostManagementMethodRepository map[uuid.UUID]HostManagementMethod

func (r testHostManagementMethodRepository) ListHostManagementMethods(context.Context, uuid.UUID) ([]HostManagementMethod, error) {
	return nil, nil
}

func (r testHostManagementMethodRepository) ListHostManagementMethodsByHostIDs(context.Context, []uuid.UUID) (map[uuid.UUID][]HostManagementMethod, error) {
	return nil, nil
}

func (r testHostManagementMethodRepository) GetHostManagementMethodByID(_ context.Context, methodID uuid.UUID) (HostManagementMethod, error) {
	method, ok := r[methodID]
	if !ok {
		return HostManagementMethod{}, errors.New("management method not found")
	}
	return method, nil
}

func (r testHostManagementMethodRepository) CreateHostManagementMethod(context.Context, uuid.UUID, CreateHostManagementMethodDTO) (HostManagementMethod, error) {
	return HostManagementMethod{}, nil
}

func (r testHostManagementMethodRepository) UpdateHostManagementMethod(_ context.Context, methodID uuid.UUID, data UpdateHostManagementMethodDTO) (HostManagementMethod, error) {
	method, ok := r[methodID]
	if !ok {
		return HostManagementMethod{}, errors.New("management method not found")
	}
	method.Name = data.Name
	method.Username = data.Username
	method.Port = data.Port
	method.Secret = data.Secret
	method.Description = data.Description
	r[methodID] = method
	return method, nil
}

func (r testHostManagementMethodRepository) DeleteHostManagementMethod(context.Context, uuid.UUID) error {
	return nil
}

type plainSecretCipher struct{}

func (plainSecretCipher) Encrypt(plainText []byte) ([]byte, error) {
	return plainText, nil
}

func (plainSecretCipher) Decrypt(cipherText []byte) ([]byte, error) {
	return cipherText, nil
}

type recordingSSHChecker struct {
	params sshcheck.Params
	result sshcheck.Result
}

func (c *recordingSSHChecker) Check(_ context.Context, params sshcheck.Params) sshcheck.Result {
	c.params = params
	return c.result
}

func TestTestSSHManagementMethodChecksDecodedPrivateKey(t *testing.T) {
	hostID := uuid.New()
	methodID := uuid.New()
	secret, err := json.Marshal(struct {
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
	}{PublicKey: "public", PrivateKey: "private"})
	if err != nil {
		t.Fatal(err)
	}
	checker := &recordingSSHChecker{
		result: sshcheck.Result{Status: sshcheck.StatusOK, Message: "ok", CheckedAt: time.Now()},
	}
	service := newTestSSHCheckService(testHostRepository{
		hostID: {ID: hostID, IP: netip.MustParseAddr("192.0.2.10")},
	}, testHostManagementMethodRepository{
		methodID: {
			ID:       methodID,
			HostID:   hostID,
			Type:     HostManagementMethodTypeSSHKey,
			Username: "root",
			Port:     2222,
			Secret:   secret,
		},
	}, checker)

	result, err := service.TestSSHManagementMethod(context.Background(), hostID, methodID)

	if err != nil {
		t.Fatal(err)
	}
	if result.Status != sshcheck.StatusOK {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if checker.params.PrivateKey != "private" {
		t.Fatalf("expected decoded private key, got %q", checker.params.PrivateKey)
	}
	if checker.params.Host != "192.0.2.10" || checker.params.Port != 2222 || checker.params.Username != "root" {
		t.Fatalf("unexpected checker params: %+v", checker.params)
	}
}

func TestTestSSHManagementMethodReturnsLookupErrors(t *testing.T) {
	hostID := uuid.New()
	methodID := uuid.New()
	otherHostID := uuid.New()
	tests := []struct {
		name    string
		hosts   testHostRepository
		methods testHostManagementMethodRepository
	}{
		{name: "missing host", hosts: nil, methods: nil},
		{name: "missing method", hosts: testHostRepository{hostID: {ID: hostID}}, methods: nil},
		{
			name:  "wrong host method pair",
			hosts: testHostRepository{hostID: {ID: hostID}},
			methods: testHostManagementMethodRepository{
				methodID: {ID: methodID, HostID: otherHostID, Type: HostManagementMethodTypeSSHPassword},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestSSHCheckService(tt.hosts, tt.methods, &recordingSSHChecker{})

			if _, err := service.TestSSHManagementMethod(context.Background(), hostID, methodID); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTestSSHManagementMethodReturnsUnsupportedStatus(t *testing.T) {
	hostID := uuid.New()
	methodID := uuid.New()
	checker := &recordingSSHChecker{result: sshcheck.Result{
		Status:    sshcheck.StatusUnsupportedMethod,
		Message:   sshcheck.Message(sshcheck.StatusUnsupportedMethod),
		CheckedAt: time.Now(),
	}}
	service := newTestSSHCheckService(testHostRepository{
		hostID: {ID: hostID, IP: netip.MustParseAddr("192.0.2.10")},
	}, testHostManagementMethodRepository{
		methodID: {
			ID:       methodID,
			HostID:   hostID,
			Type:     HostManagementMethodType("serial_console"),
			Username: "root",
			Port:     22,
		},
	}, checker)

	result, err := service.TestSSHManagementMethod(context.Background(), hostID, methodID)

	if err != nil {
		t.Fatal(err)
	}
	if result.Status != sshcheck.StatusUnsupportedMethod {
		t.Fatalf("expected unsupported method, got %s", result.Status)
	}
}

func TestUpdateHostManagementMethodKeepsPasswordWhenEmpty(t *testing.T) {
	hostID := uuid.New()
	methodID := uuid.New()
	methods := map[uuid.UUID]HostManagementMethod{methodID: {
		ID:       methodID,
		HostID:   hostID,
		Name:     "Primary",
		Type:     HostManagementMethodTypeSSHPassword,
		Username: "root",
		Port:     22,
		Secret:   []byte("old-password"),
	}}
	service := newTestSSHCheckService(testHostRepository{
		hostID: {ID: hostID, IP: netip.MustParseAddr("192.0.2.10")},
	}, testHostManagementMethodRepository(methods), &recordingSSHChecker{})

	updated, err := service.UpdateHostManagementMethod(context.Background(), hostID, methodID, UpdateHostManagementMethodInput{
		Name:     "Primary SSH",
		Username: "admin",
		Port:     2200,
	})

	if err != nil {
		t.Fatal(err)
	}
	if string(methods[methodID].Secret) != "old-password" {
		t.Fatalf("expected old secret to remain, got %q", string(methods[methodID].Secret))
	}
	if updated.Secret != nil {
		t.Fatal("expected returned method secret to be hidden")
	}
}

func newTestSSHCheckService(hosts testHostRepository, methods testHostManagementMethodRepository, checker SSHChecker) *Service {
	return NewService(NewServiceParams{
		HostRepository:                 hosts,
		HostManagementMethodRepository: methods,
		SecretCipher:                   plainSecretCipher{},
		SSHChecker:                     checker,
		Logger:                         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}
