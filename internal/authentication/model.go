package authentication

import "time"

type Profile struct {
	ID        string
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PasswordAuthentincation struct {
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
