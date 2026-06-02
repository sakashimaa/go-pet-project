package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
)

func (s *IntegrationTestSuite) TestDeleteProductByID_Success() {
	product := &domain.Product{
		Name:          "Ken Carson - More Chaos",
		Description:   "Stress Test Edition",
		Price:         5000,
		StockQuantity: 5,
		Category:      "Music",
	}

	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	err = s.ProductService.Delete(s.Ctx, id)
	s.Require().NoError(err)

	var deletedAt *time.Time
	deletedAtQuery := `
		SELECT deleted_at
		FROM products
		WHERE id = $1
	`

	err = s.DbPool.QueryRow(s.Ctx, deletedAtQuery, id).
		Scan(&deletedAt)
	s.Require().NoError(err)
	s.Require().NotNil(deletedAt)
}

func (s *IntegrationTestSuite) TestDeleteProductByIDCached() {
	product := &domain.Product{
		Name:          "Ken Carson - More Chaos",
		Description:   "Stress Test Edition",
		Price:         5000,
		StockQuantity: 5,
		Category:      "Music",
	}

	id, err := s.CachedProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	val, err := s.RedisInternalClient.Get(s.Ctx, fmt.Sprintf("product:%d", id)).Result()
	s.Require().NoError(err)
	s.Require().NotEmpty(val)

	err = s.CachedProductService.Delete(s.Ctx, id)
	s.Require().NoError(err)

	var deletedAt *time.Time
	deletedAtQuery := `
		SELECT deleted_at
		FROM products
		WHERE id = $1
	`

	err = s.DbPool.QueryRow(s.Ctx, deletedAtQuery, id).
		Scan(&deletedAt)
	s.Require().NoError(err)
	s.Require().NotNil(deletedAt)

	val, err = s.RedisInternalClient.Get(s.Ctx, fmt.Sprintf("product:%d", id)).Result()
	s.Require().Error(err)
	s.Require().Empty(val)
}

func (s *IntegrationTestSuite) TestDeleteProductByNonExistingID() {
	err := s.ProductService.Delete(s.Ctx, 999)
	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrProductNotFound)
}

func (s *IntegrationTestSuite) TestDeleteProductByID_DoubleTap() {
	product := &domain.Product{
		Name:          "Ken Carson - More Chaos",
		Description:   "Stress Test Edition",
		Price:         5000,
		StockQuantity: 5,
		Category:      "Music",
	}

	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	err = s.ProductService.Delete(s.Ctx, id)
	s.Require().NoError(err)

	err = s.ProductService.Delete(s.Ctx, id)
	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrProductNotFound)
}

func (s *IntegrationTestSuite) TestDeleteProductByIDContextCancelled() {
	ctx, cancel := context.WithCancel(s.Ctx)
	cancel()

	err := s.ProductService.Delete(ctx, 12345)

	s.Require().Error(err)
	s.Require().ErrorIs(err, context.Canceled)
}
