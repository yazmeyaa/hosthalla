package sshcheck

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Status string

const (
	StatusOK                   Status = "ok"
	StatusTimeout              Status = "timeout"
	StatusNetworkError         Status = "network_error"
	StatusAuthenticationFailed Status = "authentication_failed"
	StatusInvalidPrivateKey    Status = "invalid_private_key"
	StatusEncryptedPrivateKey  Status = "encrypted_private_key"
	StatusUnsupportedMethod    Status = "unsupported_method"
	StatusUnknownError         Status = "unknown_error"
)

const (
	MethodSSHPassword = "ssh_password"
	MethodSSHKey      = "ssh_key"
)

type Params struct {
	Host       string
	Port       uint16
	Username   string
	Method     string
	Password   string
	PrivateKey string
}

type Result struct {
	Status    Status
	Message   string
	CheckedAt time.Time
	Duration  time.Duration
}

type Checker struct {
	Timeout time.Duration
}

func (c Checker) Check(ctx context.Context, params Params) (result Result) {
	startedAt := time.Now()
	result = Result{CheckedAt: startedAt}
	defer func() {
		result.Duration = time.Since(startedAt)
	}()

	authMethods, status := AuthMethods(params)
	if status != StatusOK {
		result.Status = status
		result.Message = Message(status)
		return result
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 7 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := net.JoinHostPort(params.Host, strconv.Itoa(int(params.Port)))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		result.Status = StatusFromError(err)
		result.Message = Message(result.Status)
		return result
	}
	defer conn.Close()

	config := &ssh.ClientConfig{
		User: params.Username,
		Auth: authMethods,
		// First-version inventory check: accept any host key so saved credentials can be validated
		// without owning known_hosts storage yet.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		result.Status = StatusFromError(err)
		result.Message = Message(result.Status)
		return result
	}
	client := ssh.NewClient(clientConn, chans, reqs)
	_ = client.Close()

	result.Status = StatusOK
	result.Message = Message(StatusOK)
	return result
}

func AuthMethods(params Params) ([]ssh.AuthMethod, Status) {
	switch params.Method {
	case MethodSSHPassword:
		return []ssh.AuthMethod{ssh.Password(params.Password)}, StatusOK
	case MethodSSHKey:
		signer, err := ssh.ParsePrivateKey([]byte(params.PrivateKey))
		if err != nil {
			var passphraseErr *ssh.PassphraseMissingError
			if errors.As(err, &passphraseErr) {
				return nil, StatusEncryptedPrivateKey
			}
			return nil, StatusInvalidPrivateKey
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, StatusOK
	default:
		return nil, StatusUnsupportedMethod
	}
}

func StatusFromError(err error) Status {
	if err == nil {
		return StatusOK
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return StatusTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return StatusTimeout
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unable to authenticate"):
		return StatusAuthenticationFailed
	case strings.Contains(message, "connection refused"),
		strings.Contains(message, "no route to host"),
		strings.Contains(message, "network is unreachable"),
		strings.Contains(message, "connection reset"):
		return StatusNetworkError
	default:
		return StatusUnknownError
	}
}

func Message(status Status) string {
	switch status {
	case StatusOK:
		return "SSH authentication succeeded."
	case StatusTimeout:
		return "SSH connection timed out."
	case StatusNetworkError:
		return "Host is not reachable on the SSH port."
	case StatusAuthenticationFailed:
		return "SSH authentication failed."
	case StatusInvalidPrivateKey:
		return "Private key could not be parsed."
	case StatusEncryptedPrivateKey:
		return "Private key is encrypted with a passphrase."
	case StatusUnsupportedMethod:
		return "Management method is not supported."
	default:
		return "SSH check failed unexpectedly."
	}
}
