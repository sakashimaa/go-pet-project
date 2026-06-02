package tests

import (
	"context"
	"sync"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
)

func (s *IntegrationTestSuite) TestDecreaseStock_Success() {
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

	status, err := s.ProductService.DecreaseStock(s.Ctx, id, 5)
	s.Require().NoError(err)
	s.Require().Equal(status, "success")

	productQuery := `
		SELECT stock_quantity
		FROM products
		WHERE id = $1;
	`

	var stockQuantity int64
	err = s.DbPool.QueryRow(s.Ctx, productQuery, id).
		Scan(&stockQuantity)
	s.Require().NoError(err)
	s.Require().Equal(stockQuantity, 0)
}

func (s *IntegrationTestSuite) TestDecreaseStockInvalidInput_Failure() {
	status, err := s.ProductService.DecreaseStock(s.Ctx, 999, 999)
	s.Require().Error(err)
	s.Require().Empty(status)

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

	status, err = s.ProductService.DecreaseStock(s.Ctx, id, 100)
	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrInsufficientStock)
	s.Require().Equal("", status)

	status, err = s.ProductService.DecreaseStock(s.Ctx, id, 5)
	s.Require().NoError(err)
	s.Require().NotEmpty(status)

	var quantity int64
	stockQuantityQuery := `
		SELECT stock_quantity
		FROM products
		WHERE id = $1
	`
	err = s.DbPool.QueryRow(s.Ctx, stockQuantityQuery, id).
		Scan(&quantity)
	s.Require().NoError(err)
	s.Require().Equal(quantity, product.StockQuantity-5)
}

func (s *IntegrationTestSuite) TestDecreaseStockConcurrentRaceCondition() {
	initialStock := int64(100)
	product := &domain.Product{
		Name:          "Ken Carson - More Chaos",
		Description:   "Stress Test Edition",
		Price:         5000,
		StockQuantity: initialStock,
		Category:      "Music",
	}

	id, err := s.ProductService.Create(s.Ctx, product)
	s.Require().NoError(err)
	s.Require().NotZero(id)

	concurrency := int(initialStock)

	errCh := make(chan error, concurrency)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			_, err := s.ProductService.DecreaseStock(context.Background(), id, 1)
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		s.Require().NoError(err, "Race Condition: Request failed unexpectedly")
	}

	var finalStock int64
	err = s.DbPool.QueryRow(s.Ctx, "SELECT stock_quantity FROM products WHERE id = $1", id).
		Scan(&finalStock)

	s.Require().NoError(err)
	s.Require().Equal(int64(0), finalStock, "Final stock is NOT zero!")
}

func (s *IntegrationTestSuite) TestDecreaseStockContextCancelled() {
	ctx, cancel := context.WithCancel(s.Ctx)
	cancel()

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

	status, err := s.ProductService.DecreaseStock(ctx, id, 5)
	s.Require().Empty(status)
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.Canceled)
}
