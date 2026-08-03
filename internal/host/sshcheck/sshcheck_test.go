package sshcheck

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestAuthMethodsBuildsPasswordAuth(t *testing.T) {
	authMethods, status := AuthMethods(Params{Method: MethodSSHPassword, Password: "secret"})

	if status != StatusOK {
		t.Fatalf("expected ok, got %s", status)
	}
	if len(authMethods) != 1 {
		t.Fatalf("expected one auth method, got %d", len(authMethods))
	}
}

func TestAuthMethodsRejectsInvalidPrivateKey(t *testing.T) {
	_, status := AuthMethods(Params{Method: MethodSSHKey, PrivateKey: "not a key"})

	if status != StatusInvalidPrivateKey {
		t.Fatalf("expected invalid private key, got %s", status)
	}
}

func TestAuthMethodsBuildsEd25519KeyAuth(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	key := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	authMethods, status := AuthMethods(Params{Method: MethodSSHKey, PrivateKey: string(key)})

	if status != StatusOK {
		t.Fatalf("expected ok, got %s", status)
	}
	if len(authMethods) != 1 {
		t.Fatalf("expected one auth method, got %d", len(authMethods))
	}
}

func TestAuthMethodsReportsEncryptedEd25519Key(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "test", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	key := pem.EncodeToMemory(block)

	_, status := AuthMethods(Params{Method: MethodSSHKey, PrivateKey: string(key)})

	if status != StatusEncryptedPrivateKey {
		t.Fatalf("expected encrypted private key, got %s", status)
	}
}

func TestStatusFromError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status Status
	}{
		{name: "timeout", err: context.DeadlineExceeded, status: StatusTimeout},
		{name: "auth", err: errors.New("ssh: unable to authenticate, attempted methods [none]"), status: StatusAuthenticationFailed},
		{name: "network", err: errors.New("dial tcp: connect: connection refused"), status: StatusNetworkError},
		{name: "unknown", err: errors.New("ssh: handshake failed"), status: StatusUnknownError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusFromError(tt.err); got != tt.status {
				t.Fatalf("expected %s, got %s", tt.status, got)
			}
		})
	}
}
