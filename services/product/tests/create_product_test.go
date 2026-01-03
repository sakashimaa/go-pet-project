package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
)

func (s *IntegrationTestSuite) TestCreateProduct_Success() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}
	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	var dbName string
	var dbPrice int64

	productQuery := `
		SELECT name, price
		FROM products
		WHERE id = $1
	`
	err = s.DbPool.QueryRow(s.Ctx, productQuery, id).
		Scan(&dbName, &dbPrice)
	s.Require().NoError(err)
	s.Require().Equal(product.Name, dbName)
	s.Require().Equal(product.Price, dbPrice)

	publishedAtQuery := `
		SELECT published_at
		FROM outbox
		WHERE aggregate_id = $1 AND event_type = 'ProductCreated'
	`

	s.Require().Eventually(func() bool {
		var publishedAt *time.Time

		err = s.DbPool.QueryRow(s.Ctx, publishedAtQuery, fmt.Sprintf("%d", id)).
			Scan(&publishedAt)
		if err != nil || publishedAt == nil {
			return false
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *IntegrationTestSuite) TestCreateProductUnique_Failed() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}
	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	id, err = s.ProductService.Create(s.Ctx, product)
	s.Require().Error(err)
	s.Require().Zero(id)
}

func (s *IntegrationTestSuite) TestCreateProductContextTimeout_Failed() {
	ctxTimeout, cancel := context.WithTimeout(s.Ctx, 1*time.Nanosecond)
	defer cancel()

	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}

	id, err := s.ProductService.Create(ctxTimeout, product)
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.DeadlineExceeded)
	s.Require().Zero(id)
}
