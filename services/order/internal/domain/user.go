package domain

type User struct {
	ID    int64  `db:"id"`
	Email string `db:"email"`
}
