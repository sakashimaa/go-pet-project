package domain

type UserRegisteredEvent struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}
