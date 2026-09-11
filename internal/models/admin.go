package models

import "time"

type AdminUser struct {
	ID                 int       `json:"id"`
	Email              string    `json:"email"`
	PasswordHash       string    `json:"-"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type SecuritySettings struct {
	AuthEnabled bool      `json:"auth_enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}
