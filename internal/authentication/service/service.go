package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidProfile  = errors.New("invalid profile")
)

const (
	minPasswordLength = 8
	passwordHashCost  = bcrypt.DefaultCost
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

type Service struct {
	profileRepository                storage.ProfileRepository
	passwordAuthenticationRepository storage.PasswordAuthenticationRepository
	sessionRepository                storage.SessionRepository
}

func New(
	profileRepository storage.ProfileRepository,
	passwordAuthenticationRepository storage.PasswordAuthenticationRepository,
	sessionRepository storage.SessionRepository,
) *Service {
	return &Service{
		profileRepository:                profileRepository,
		passwordAuthenticationRepository: passwordAuthenticationRepository,
		sessionRepository:                sessionRepository,
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

func (s *Service) SetPassword(ctx context.Context, data SetPasswordDTO) (authentication.PasswordAuthentincation, error) {
	passwordHash, err := HashPassword(data.Password)
	if err != nil {
		return authentication.PasswordAuthentincation{}, err
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
