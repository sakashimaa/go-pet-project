package domain

import "time"

type User struct {
	ID                  int64     `db:"id"`
	Email               string    `db:"email"`
	Password            string    `db:"password_hash"`
	ActivationToken     string    `db:"activation_token"`
	IsActivated         bool      `db:"is_activated"`
	ForgotPasswordToken string    `db:"forgot_password_token"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}
