package storage

import (
	"context"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
)

type CreateProfileDTO struct {
	Username string
}

type CreatePasswordAuthenticationDTO struct {
	ProfileID    string
	PasswordHash string
}

type CreateSessionDTO struct {
	ProfileID string
}

type ProfileRepository interface {
	CreateProfile(ctx context.Context, data CreateProfileDTO) (authentication.Profile, error)
	GetProfileByID(ctx context.Context, id string) (authentication.Profile, error)
	GetProfileByUsername(ctx context.Context, username string) (authentication.Profile, error)
	UpdateProfile(ctx context.Context, profile *authentication.Profile) error
	DeleteProfile(ctx context.Context, id string) error
}

type PasswordAuthenticationRepository interface {
	CreatePasswordAuthentication(ctx context.Context, data CreatePasswordAuthenticationDTO) (authentication.PasswordAuthentincation, error)
	GetPasswordAuthenticationByID(ctx context.Context, id string) (authentication.PasswordAuthentincation, error)
	GetPasswordAuthenticationByUsername(ctx context.Context, username string) (authentication.PasswordAuthentincation, error)
}

type SessionRepository interface {
	CreateSession(ctx context.Context, data CreateSessionDTO) (authentication.Session, error)
	GetSessionByID(ctx context.Context, id string) (authentication.Session, error)
	GetSessionByProfileID(ctx context.Context, profileID string) (authentication.Session, error)
}
