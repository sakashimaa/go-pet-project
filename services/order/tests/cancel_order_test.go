package tests

import (
	"errors"
	"fmt"
	"time"

	"github.com/sakashimaa/go-pet-project/order/internal/repository"
	"github.com/sakashimaa/go-pet-project/pkg/domain"
)

func (s *IntegrationTestSuite) TestCancelOrder_Success() {
	s.seedData(999, "test@example.com")

	resp := s.createOrder(999)

	err := s.OrderService.CancelOrder(s.Ctx, &domain.PaymentFailedEvent{
		OrderID:   resp.OrderId,
		PaymentID: 999,
		Amount:    999,
		FailedAt:  time.Now(),
	})
	s.Require().NoError(err)

	orderQuery := `
		SELECT status
		FROM orders
		WHERE id = $1
	`

	var status string
	err = s.DbPool.QueryRow(s.Ctx, orderQuery, resp.OrderId).
		Scan(&status)
	s.Require().NoError(err)
	s.Require().Equal(status, "cancelled")

	publishedAtQuery := `
		SELECT published_at, event_type
		FROM outbox
		WHERE aggregate_id = $1 AND event_type = 'OrderCancelled'
	`

	s.Require().Eventually(func() bool {
		var publishedAt *time.Time
		var eventType string

		err = s.DbPool.QueryRow(s.Ctx, publishedAtQuery, fmt.Sprintf("%d", resp.OrderId)).
			Scan(&publishedAt, &eventType)
		if err != nil || publishedAt == nil || eventType != "OrderCancelled" {
			return false
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *IntegrationTestSuite) TestCancelOrder_NotFound() {
	err := s.OrderService.CancelOrder(s.Ctx, &domain.PaymentFailedEvent{
		OrderID:   999,
		PaymentID: 999,
		Amount:    999,
		FailedAt:  time.Now(),
	})

	s.Require().Error(err)
	s.Require().True(errors.Is(err, repository.ErrOrderNotFound))
}

func (s *IntegrationTestSuite) TestCancelOrder_Idempotency() {
	s.seedData(999, "test@example.com")
	resp := s.createOrder(999)

	event := &domain.PaymentFailedEvent{
		OrderID:   resp.OrderId,
		PaymentID: 999,
		Amount:    999,
		FailedAt:  time.Now(),
	}

	err := s.OrderService.CancelOrder(s.Ctx, event)
	s.Require().NoError(err)

	err = s.OrderService.CancelOrder(s.Ctx, event)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TestCancelOrder_FailIfPaid() {
	s.seedData(999, "test@example.com")
	resp := s.createOrder(999)

	err := s.OrderService.ChangeOrderStatusPaymentSucceeded(s.Ctx, &domain.PaymentSucceededEvent{
		OrderID:   resp.OrderId,
		PaymentID: 999,
		Amount:    999,
		PaidAt:    time.Now(),
	})
	s.Require().NoError(err)

	err = s.OrderService.CancelOrder(s.Ctx, &domain.PaymentFailedEvent{
		OrderID:   resp.OrderId,
		PaymentID: 999,
		Amount:    999,
		FailedAt:  time.Now(),
	})
	s.Require().Error(err)
}
