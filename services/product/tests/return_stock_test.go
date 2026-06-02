package tests

import (
	"context"
	"crypto/rand"
	"math/big"

	domain2 "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
)

func (s *IntegrationTestSuite) TestReturnStock_Success() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}

	id, _ := s.ProductService.Create(s.Ctx, product)
	s.Require().NotZero(id)

	orderId, err := rand.Int(rand.Reader, big.NewInt(1000))

	s.Require().NoError(err)
	err = s.ProductService.ReturnStock(s.Ctx, &domain2.OrderCancelledEvent{
		OrderID: orderId.Int64(),
		Items: []domain2.OrderItem{
			{
				OrderID:   orderId.Int64(),
				ProductID: id,
				Name:      product.Name,
				Price:     product.Price,
				Quantity:  int32(2),
			},
		},
	})
	s.Require().NoError(err)

	var stockQuantity int64
	err = s.DbPool.QueryRow(s.Ctx, "SELECT stock_quantity FROM products WHERE id = $1", id).
		Scan(&stockQuantity)
	s.Require().NoError(err)
	s.Require().Equal(product.StockQuantity+2, stockQuantity)
}

func (s *IntegrationTestSuite) TestReturnStockNotFound() {
	orderId, err := rand.Int(rand.Reader, big.NewInt(1000))
	s.Require().NoError(err)

	err = s.ProductService.ReturnStock(s.Ctx, &domain2.OrderCancelledEvent{
		OrderID: orderId.Int64(),
		Items: []domain2.OrderItem{
			{
				OrderID:   orderId.Int64(),
				ProductID: 999,
				Name:      "fake name",
				Price:     999,
				Quantity:  int32(999),
			},
		},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrProductNotFound)
}

func (s *IntegrationTestSuite) TestReturnStockContextCancelled() {
	ctx, cancel := context.WithCancel(s.Ctx)
	cancel()

	err := s.ProductService.ReturnStock(ctx, &domain2.OrderCancelledEvent{})
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *IntegrationTestSuite) TestReturnStockInvalidInput() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}

	id, _ := s.ProductService.Create(s.Ctx, product)
	s.Require().NotZero(id)

	orderId, err := rand.Int(rand.Reader, big.NewInt(1000))
	s.Require().NoError(err)

	err = s.ProductService.ReturnStock(s.Ctx, &domain2.OrderCancelledEvent{
		OrderID: orderId.Int64(),
		Items: []domain2.OrderItem{
			{
				OrderID:   orderId.Int64(),
				ProductID: id,
				Name:      product.Name,
				Price:     product.Price,
				Quantity:  int32(-1),
			},
		},
	})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrInvalidInput)
}

func (s *IntegrationTestSuite) TestReturnStock_Atomicity_Rollback() {
	prodA := &domain.Product{
		Name:          "Item A",
		Price:         100,
		StockQuantity: 10,
	}

	idA, _ := s.ProductService.Create(s.Ctx, prodA)
	orderId, _ := rand.Int(rand.Reader, big.NewInt(1000))

	err := s.ProductService.ReturnStock(s.Ctx, &domain2.OrderCancelledEvent{
		OrderID: orderId.Int64(),
		Items: []domain2.OrderItem{
			{
				ProductID: idA,
				Quantity:  5,
			},
			{
				ProductID: 99999,
				Quantity:  5,
			},
		},
	})

	s.Require().Error(err)

	var stockA int64
	err = s.DbPool.QueryRow(s.Ctx, "SELECT stock_quantity FROM products WHERE id = $1", idA).
		Scan(&stockA)
	s.Require().NoError(err)
	s.Require().Equal(int64(10), stockA, "Transaction leak! Stock increased despite partial failure")
}
