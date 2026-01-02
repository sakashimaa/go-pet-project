package domain

type UserRegisteredEvent struct {
	UserID          int64  `json:"user_id"`
	Email           string `json:"email"`
	ActivationToken string `json:"activation_token"`
	Event           string `json:"event"`
	EventID         int64  `json:"event_id"`
}

type UserForgotPasswordEvent struct {
	Email               string `json:"email"`
	ForgotPasswordToken string `json:"forgot_password_token"`
	Event               string `json:"event"`
	EventID             int64  `json:"event_id"`
}

type UserResetPasswordEvent struct {
	Email   string `json:"email"`
	Event   string `json:"event"`
	EventID int64  `json:"event_id"`
}
