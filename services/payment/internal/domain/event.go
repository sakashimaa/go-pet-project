package domain

import "time"

type InventoryReservedEvent struct {
	OrderID    int64     `json:"order_id"`
	UserID     int64     `json:"user_id"`
	Amount     int64     `json:"amount"`
	ReservedAt time.Time `json:"reserved_at"`
}
