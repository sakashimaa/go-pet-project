package tests

import (
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
)

func (s *IntegrationTestSuite) TestProductList_Success() {
	productsDataSet := []domain.Product{
		{
			Name:          "A Great Chaos Vinyl",
			Description:   "Best album vinyl",
			Price:         9999,
			StockQuantity: 5,
			ImageUrl:      "https://external-preview.redd.it/a-great-chaos-vinyl-ken-carson-official-store-v0-VYScH2jkLZ7YH5UHw2jzbvhdK5j51QmhRVBeMgaQB8U.jpg?auto=webp&s=1914208b446acb94dcef73fa9ef5ad331f23e4f6",
			Category:      "Music",
		},
		{
			Name:          "黒波・混沌 Edition",
			Description:   "真のサムライのための武器。Sony XM5で集中せよ。⛩️",
			Price:         15000,
			StockQuantity: 1,
			ImageUrl:      "https://example.com/kuronami_jp.jpg",
			Category:      "限定グッズ",
		},
	}

	var id int64
	var err error
	for _, product := range productsDataSet {
		id, err = s.CachedProductService.Create(s.Ctx, &product)
		s.Require().NoError(err)
		s.Require().NotZero(id)
	}

	productsList, ttl, err := s.CachedProductService.List(s.Ctx, 10, 0, "")
	s.Require().NoError(err)
	s.Require().Equal(int(ttl), len(productsDataSet))
	s.Require().Equal(len(productsDataSet), len(productsList))

	productsMap := make(map[string]domain.Product)
	for _, p := range productsList {
		productsMap[p.Name] = p
	}

	for _, expected := range productsDataSet {
		actual, exists := productsMap[expected.Name]

		s.Require().True(exists, "Product %s not found in response", expected.Name)
		s.Require().Equal(expected.Description, actual.Description)
		s.Require().Equal(expected.Price, actual.Price)
		s.Require().Equal(expected.StockQuantity, actual.StockQuantity)
		s.Require().Equal(expected.ImageUrl, actual.ImageUrl)
		s.Require().Equal(expected.Category, actual.Category)
	}
}
