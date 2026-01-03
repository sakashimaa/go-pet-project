package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
)

func (s *IntegrationTestSuite) TestFindById_Success() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
		Category:      "Music",
	}

	id, err := s.CachedProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	created, err := s.CachedProductService.FindByID(s.Ctx, id)
	s.Require().NoError(err)
	s.Require().Equal(created.ID, id)
	s.Require().Equal(created.Name, product.Name)
	s.Require().Equal(created.Description, product.Description)
	s.Require().Equal(created.Price, product.Price)
	s.Require().Equal(created.StockQuantity, product.StockQuantity)
	s.Require().Equal(created.ImageUrl, product.ImageUrl)
	s.Require().Equal(created.Category, product.Category)

	val, err := s.RedisInternalClient.Get(s.Ctx, fmt.Sprintf("product:%d", id)).Result()
	s.Require().NoError(err)
	s.Require().NotEmpty(val)

	productJP := &domain.Product{
		Name:          "黒波・混沌 Edition",
		Description:   "真のサムライのための武器。Sony XM5で集中せよ。⛩️",
		Price:         15000,
		StockQuantity: 1,
		ImageUrl:      "https://example.com/kuronami_jp.jpg",
		Category:      "限定グッズ",
	}

	jpId, err := s.ProductService.Create(s.Ctx, productJP)
	s.Require().NoError(err)

	japaneseDbProduct, err := s.ProductService.FindByID(s.Ctx, jpId)
	s.Require().NoError(err)
	s.Require().Equal(jpId, japaneseDbProduct.ID)
	s.Require().Equal(productJP.Name, japaneseDbProduct.Name)
	s.Require().Equal(productJP.Description, japaneseDbProduct.Description)
	s.Require().Equal(productJP.Price, japaneseDbProduct.Price)
	s.Require().Equal(productJP.StockQuantity, japaneseDbProduct.StockQuantity)
	s.Require().Equal(productJP.ImageUrl, japaneseDbProduct.ImageUrl)
	s.Require().Equal(productJP.Category, japaneseDbProduct.Category)
}

func (s *IntegrationTestSuite) TestFindByID_Failure() {
	product, err := s.ProductService.FindByID(s.Ctx, 999)
	s.Require().Error(err)
	s.Require().Nil(product)
	s.Require().ErrorIs(err, repository.ErrProductNotFound)
}

func (s *IntegrationTestSuite) TestFindByIDZeroFields_Success() {
	product := &domain.Product{
		Name:          "A Great Chaos Vinyl",
		Description:   "Best album vinyl",
		Price:         9999,
		StockQuantity: 5,
		ImageUrl:      "",
		Category:      "",
	}

	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	dbProduct, err := s.ProductService.FindByID(s.Ctx, id)
	s.Require().NoError(err)
	s.Require().NotNil(dbProduct)
	s.Require().Empty(dbProduct.ImageUrl)
	s.Require().Empty(dbProduct.Category)
}

func (s *IntegrationTestSuite) TestFindByID_ContextTimeout() {
	product := &domain.Product{
		Name:  "timeout test product",
		Price: 100,
	}
	id, _ := s.ProductService.Create(s.Ctx, product)

	timeoutCtx, cancel := context.WithTimeout(s.Ctx, 1*time.Microsecond)
	defer cancel()

	val, err := s.ProductService.FindByID(timeoutCtx, id)
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.DeadlineExceeded)
	s.Require().Nil(val)
}
