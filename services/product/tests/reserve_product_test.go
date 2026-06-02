package tests

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
)

func (s *IntegrationTestSuite) TestReserveProduct_Success() {
	orderId, err := rand.Int(rand.Reader, big.NewInt(1000))
	s.Require().NoError(err)

	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}

	id, err := s.ProductService.Create(context.Background(), product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	err = s.ProductService.ReserveProduct(s.Ctx, &domain.OrderCreatedEvent{
		OrderID: orderId.Int64(),
		UserID:  1,
		Items: []domain.OrderItemEvent{
			{
				ProductID: id,
				Quantity:  5,
			},
		},
	})
	s.Require().NoError(err)

	var stockQuantity int64
	stockQuantityQuery := `
		SELECT stock_quantity
		FROM products
		WHERE id = $1
	`
	err = s.DbPool.QueryRow(s.Ctx, stockQuantityQuery, id).
		Scan(&stockQuantity)
	s.Require().NoError(err)
	s.Require().Equal(int64(0), stockQuantity)

	publishedAtQuery := `
		SELECT published_at
		FROM outbox
		WHERE aggregate_id = $1 AND event_type = 'InventoryReserved'
	`

	s.Require().Eventually(func() bool {
		var publishedAt *time.Time
		err = s.DbPool.QueryRow(context.Background(), publishedAtQuery, fmt.Sprintf("%d", orderId.Int64())).
			Scan(&publishedAt)

		if err != nil || publishedAt == nil {
			return false
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *IntegrationTestSuite) TestReserveProduct_InsufficientStock_Fail() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 2,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}
	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	err = s.ProductService.ReserveProduct(s.Ctx, &domain.OrderCreatedEvent{
		OrderID: 999,
		UserID:  1,
		Items: []domain.OrderItemEvent{
			{ProductID: id, Quantity: 5},
		},
	})

	s.Require().Error(err)

	var stockQuantity int64
	err = s.DbPool.QueryRow(s.Ctx, "SELECT stock_quantity FROM products WHERE id = $1", id).
		Scan(&stockQuantity)
	s.Require().NoError(err)
	s.Require().Equal(int64(2), stockQuantity, "Stock should not change on failure")
}

func (s *IntegrationTestSuite) TestReserveProduct_Atomicity_Rollback() {
	prodA := &domain.Product{Name: "Item A", Price: 10, StockQuantity: 100}
	idA, _ := s.ProductService.Create(s.Ctx, prodA)

	prodB := &domain.Product{Name: "Item B", Price: 10, StockQuantity: 1}
	idB, _ := s.ProductService.Create(s.Ctx, prodB)

	err := s.ProductService.ReserveProduct(s.Ctx, &domain.OrderCreatedEvent{
		OrderID: 888,
		UserID:  1,
		Items: []domain.OrderItemEvent{
			{ProductID: idA, Quantity: 5},
			{ProductID: idB, Quantity: 5},
		},
	})

	s.Require().Error(err)

	var stockA int64
	err = s.DbPool.QueryRow(s.Ctx, "SELECT stock_quantity FROM products WHERE id = $1", idA).
		Scan(&stockA)
	s.Require().NoError(err)
	s.Require().Equal(int64(100), stockA)
}

func (s *IntegrationTestSuite) TestReserveProduct_CancelledContext() {
	ctx, cancel := context.WithCancel(s.Ctx)
	cancel()

	err := s.ProductService.ReserveProduct(ctx, &domain.OrderCreatedEvent{
		OrderID: 111,
		UserID:  1,
		Items: []domain.OrderItemEvent{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 5},
		},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *IntegrationTestSuite) TestReserveProductNotFound() {
	err := s.ProductService.ReserveProduct(s.Ctx, &domain.OrderCreatedEvent{
		OrderID: 111,
		UserID:  1,
		Items: []domain.OrderItemEvent{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 5},
		},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrProductNotFound)
}
