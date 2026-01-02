package domain

import "time"

type OrderItemEvent struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type OrderCreatedEvent struct {
	OrderID int64            `json:"order_id"`
	UserID  int64            `json:"user_id"`
	Items   []OrderItemEvent `json:"items"`
}

type InventoryReservedEvent struct {
	OrderID    int64     `json:"order_id"`
	UserID     int64     `json:"user_id"`
	Amount     int64     `json:"amount"`
	ReservedAt time.Time `json:"reserved_at"`
}
