package domain

import "time"

type Payment struct {
	ID            int64  `db:"id"`
	OrderID       int64  `db:"order_id"`
	UserID        int64  `db:"user_id"`
	Status        string `db:"status"`
	Amount        int64  `db:"amount"`
	TransactionID string `db:"transaction_id"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
