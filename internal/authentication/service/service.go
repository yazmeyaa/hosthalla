package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidProfile  = errors.New("invalid profile")
	ErrInvalidToken    = errors.New("invalid token")
)

const (
	minPasswordLength    = 8
	passwordHashCost     = bcrypt.DefaultCost
	apiTokenBytesSize    = 32
	apiTokenPrefixLen    = 8
	apiTokenDefaultScope = "hosts:register"
)

type CreateUserDTO struct {
	Username string
	Password string
}

type UpdateUserDTO struct {
	ID       string
	Username string
}

type SetPasswordDTO struct {
	ProfileID string
	Password  string
}

type CreateSessionDTO struct {
	ProfileID string
}

type CreateAPITokenDTO struct {
	ProfileID string
	Name      string
	Scopes    []string
	ExpiresIn time.Duration
}

type CreateAPITokenResult struct {
	Token      authentication.APIToken
	PlainToken string
}

type Service struct {
	profileRepository                storage.ProfileRepository
	passwordAuthenticationRepository storage.PasswordAuthenticationRepository
	sessionRepository                storage.SessionRepository
	apiTokenRepository               storage.APITokenRepository
}

type NewParams struct {
	ProfileRepository                storage.ProfileRepository
	PasswordAuthenticationRepository storage.PasswordAuthenticationRepository
	SessionRepository                storage.SessionRepository
	APITokenRepository               storage.APITokenRepository
}

func New(params NewParams) *Service {
	return &Service{
		profileRepository:                params.ProfileRepository,
		passwordAuthenticationRepository: params.PasswordAuthenticationRepository,
		sessionRepository:                params.SessionRepository,
		apiTokenRepository:               params.APITokenRepository,
	}
}

func (s *Service) CreateUser(ctx context.Context, data CreateUserDTO) (authentication.Profile, error) {
	username := strings.TrimSpace(data.Username)
	if username == "" {
		return authentication.Profile{}, ErrInvalidUsername
	}

	passwordHash, err := HashPassword(data.Password)
	if err != nil {
		return authentication.Profile{}, err
	}

	profile, err := s.profileRepository.CreateProfile(ctx, storage.CreateProfileDTO{
		Username: username,
	})
	if err != nil {
		return authentication.Profile{}, err
	}

	_, err = s.passwordAuthenticationRepository.CreatePasswordAuthentication(ctx, storage.CreatePasswordAuthenticationDTO{
		ProfileID:    profile.ID,
		PasswordHash: passwordHash,
	})
	if err != nil {
		// Avoid leaving orphaned users when password setup fails.
		if rollbackErr := s.profileRepository.DeleteProfile(ctx, profile.ID); rollbackErr != nil {
			return authentication.Profile{}, errors.Join(err, rollbackErr)
		}
		return authentication.Profile{}, err
	}

	return profile, nil
}

func (s *Service) GetProfileByID(ctx context.Context, id string) (authentication.Profile, error) {
	return s.profileRepository.GetProfileByID(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]authentication.Profile, error) {
	return s.profileRepository.ListProfiles(ctx)
}

func (s *Service) GetProfileByUsername(ctx context.Context, username string) (authentication.Profile, error) {
	return s.profileRepository.GetProfileByUsername(ctx, username)
}

func (s *Service) UpdateUser(ctx context.Context, data UpdateUserDTO) (authentication.Profile, error) {
	username := strings.TrimSpace(data.Username)
	if username == "" {
		return authentication.Profile{}, ErrInvalidUsername
	}

	profile, err := s.profileRepository.GetProfileByID(ctx, data.ID)
	if err != nil {
		return authentication.Profile{}, err
	}

	profile.Username = username
	if err := s.profileRepository.UpdateProfile(ctx, &profile); err != nil {
		return authentication.Profile{}, err
	}
	return profile, nil
}

func (s *Service) DeleteUser(ctx context.Context, profileID string) error {
	return s.profileRepository.DeleteProfile(ctx, profileID)
}

func (s *Service) SetPassword(ctx context.Context, data SetPasswordDTO) (authentication.PasswordAuthentication, error) {
	passwordHash, err := HashPassword(data.Password)
	if err != nil {
		return authentication.PasswordAuthentication{}, err
	}

	return s.passwordAuthenticationRepository.CreatePasswordAuthentication(ctx, storage.CreatePasswordAuthenticationDTO{
		ProfileID:    data.ProfileID,
		PasswordHash: passwordHash,
	})
}

func (s *Service) ValidatePassword(ctx context.Context, username, plainPassword string) (bool, error) {
	passwordAuth, err := s.passwordAuthenticationRepository.GetPasswordAuthenticationByUsername(ctx, username)
	if err != nil {
		return false, err
	}
	return ComparePassword(passwordAuth.PasswordHash, plainPassword), nil
}

func (s *Service) CreateSession(ctx context.Context, data CreateSessionDTO) (authentication.Session, error) {
	profileID := strings.TrimSpace(data.ProfileID)
	if profileID == "" {
		return authentication.Session{}, ErrInvalidProfile
	}

	return s.sessionRepository.CreateSession(ctx, storage.CreateSessionDTO{
		ProfileID: profileID,
	})
}

func (s *Service) GetSessionByID(ctx context.Context, id string) (authentication.Session, error) {
	return s.sessionRepository.GetSessionByID(ctx, id)
}

func (s *Service) GetSessionByProfileID(ctx context.Context, profileID string) (authentication.Session, error) {
	return s.sessionRepository.GetSessionByProfileID(ctx, profileID)
}

func (s *Service) DeleteSession(ctx context.Context, id string) error {
	return s.sessionRepository.DeleteSession(ctx, strings.TrimSpace(id))
}

func (s *Service) CreateAPIToken(ctx context.Context, data CreateAPITokenDTO) (CreateAPITokenResult, error) {
	profileID := strings.TrimSpace(data.ProfileID)
	if profileID == "" {
		return CreateAPITokenResult{}, ErrInvalidProfile
	}

	name := strings.TrimSpace(data.Name)
	if name == "" {
		return CreateAPITokenResult{}, fmt.Errorf("%w: name is required", ErrInvalidToken)
	}

	scopes := normalizeScopes(data.Scopes)
	if len(scopes) == 0 {
		scopes = []string{apiTokenDefaultScope}
	}

	rawToken, err := generateRawToken()
	if err != nil {
		return CreateAPITokenResult{}, err
	}
	plainToken := "hht_" + rawToken
	tokenHash := hashToken(plainToken)

	var expiresAt *time.Time
	if data.ExpiresIn > 0 {
		value := time.Now().Add(data.ExpiresIn)
		expiresAt = &value
	}

	token, err := s.apiTokenRepository.CreateAPIToken(ctx, storage.CreateAPITokenDTO{
		ProfileID: profileID,
		Name:      name,
		Prefix:    rawToken[:apiTokenPrefixLen],
		Hash:      tokenHash,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return CreateAPITokenResult{}, err
	}

	return CreateAPITokenResult{
		Token:      token,
		PlainToken: plainToken,
	}, nil
}

func (s *Service) GetAPITokenByID(ctx context.Context, id string) (authentication.APIToken, error) {
	return s.apiTokenRepository.GetAPITokenByID(ctx, id)
}

func (s *Service) ListAPITokens(ctx context.Context) ([]authentication.APIToken, error) {
	return s.apiTokenRepository.ListAPITokens(ctx)
}

func (s *Service) ListAPITokensByProfileID(ctx context.Context, profileID string) ([]authentication.APIToken, error) {
	return s.apiTokenRepository.ListAPITokensByProfileID(ctx, profileID)
}

func (s *Service) RevokeAPIToken(ctx context.Context, id string) error {
	return s.apiTokenRepository.RevokeAPIToken(ctx, id)
}

func HashPassword(plainPassword string) (string, error) {
	if len(plainPassword) < minPasswordLength {
		return "", ErrInvalidPassword
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), passwordHashCost)
	if err != nil {
		return "", err
	}
	return string(passwordHash), nil
}

func ComparePassword(passwordHash, plainPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(plainPassword)) == nil
}

func generateRawToken() (string, error) {
	buf := make([]byte, apiTokenBytesSize)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeScopes(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		normalized := strings.ToLower(strings.TrimSpace(scope))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
