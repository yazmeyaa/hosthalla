package authentication

import "time"

type Profile struct {
	ID        string
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PasswordAuthentication struct {
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string
	ProfileID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIToken struct {
	ID        string
	ProfileID string

	Name string

	Prefix string
	Hash   string

	Scopes []string

	LastUsedAt *time.Time

	CreatedAt time.Time
	ExpiresAt *time.Time
	RevokedAt *time.Time
}
