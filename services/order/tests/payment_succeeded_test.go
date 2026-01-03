package tests

import (
	"errors"
	"fmt"
	"time"

	"github.com/sakashimaa/go-pet-project/order/internal/repository"
	"github.com/sakashimaa/go-pet-project/pkg/domain"
)

func (s *IntegrationTestSuite) TestPaymentSucceeded_Success() {
	s.seedData(999, "test@example.com")
	resp := s.createOrder(999)

	err := s.OrderService.ChangeOrderStatusPaymentSucceeded(s.Ctx, &domain.PaymentSucceededEvent{
		PaymentID: 999,
		OrderID:   resp.OrderId,
		Amount:    999,
		PaidAt:    time.Now(),
	})
	s.Require().NoError(err)

	query := `
		SELECT status
		FROM orders
		WHERE id = $1
	`
	var status string
	err = s.DbPool.QueryRow(s.Ctx, query, resp.OrderId).Scan(&status)
	s.Require().NoError(err)
	s.Require().NotEmpty(status)

	s.Require().Equal(status, "paid")
}

func (s *IntegrationTestSuite) TestPaymentFailed_Failure() {
	err := s.OrderService.ChangeOrderStatusPaymentSucceeded(s.Ctx, &domain.PaymentSucceededEvent{
		PaymentID: 999,
		OrderID:   999999,
		Amount:    999,
		PaidAt:    time.Now(),
	})
	s.Require().Error(err)

	fmt.Printf("%v", err)
	s.Require().True(errors.Is(err, repository.ErrOrderNotFound))
}
