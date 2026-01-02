package domain

import "time"

type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "new"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusShipped   OrderStatus = "shipped"
)

type Order struct {
	ID       int64       `db:"id"`
	UserID   int64       `db:"user_id"`
	Status   OrderStatus `db:"status"`
	Items    []OrderItem `db:"items"`
	TotalSum int64       `db:"total_sum"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type OrderItem struct {
	ID        int64  `db:"id"`
	OrderID   int64  `db:"order_id"`
	ProductID int64  `db:"product_id"`
	Name      string `db:"name"`
	Price     int64  `db:"price"`
	Quantity  int32  `db:"quantity"`
}

func (o *Order) CalculateTotal() {
	var total int64
	for _, item := range o.Items {
		total += item.Price * int64(item.Quantity)
	}
	o.TotalSum = total
}
