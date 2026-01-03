package domain

import (
	"time"

	pb "github.com/sakashimaa/go-pet-project/proto/order"
)

type PaymentSucceededEvent struct {
	OrderID   int64     `json:"order_id"`
	PaymentID int64     `json:"payment_id"`
	Amount    int64     `json:"amount"`
	PaidAt    time.Time `json:"paid_at"`
}

type PaymentFailedEvent struct {
	OrderID   int64     `json:"order_id"`
	PaymentID int64     `json:"payment_id"`
	Amount    int64     `json:"amount"`
	FailedAt  time.Time `json:"failed_at"`
}

type OrderItem struct {
	ID        int64  `db:"id"`
	OrderID   int64  `db:"order_id"`
	ProductID int64  `db:"product_id"`
	Name      string `db:"name"`
	Price     int64  `db:"price"`
	Quantity  int32  `db:"quantity"`
}

type OrderCancelledEvent struct {
	OrderID int64       `json:"order_id"`
	Items   []OrderItem `json:"items"`
}

func (i *OrderItem) ToPB() *pb.OrderItem {
	return &pb.OrderItem{
		ProductId: i.ProductID,
		Name:      i.Name,
		Price:     i.Price,
		Quantity:  i.Quantity,
	}
}
