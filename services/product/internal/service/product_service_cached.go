package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	generalDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
)

type cachedProductService struct {
	next        ProductService
	redisClient *redis.Client
	cacheTTL    time.Duration
}

func (s *cachedProductService) ReserveProduct(ctx context.Context, event *domain.OrderCreatedEvent) error {
	return s.next.ReserveProduct(ctx, event)
}

func (s *cachedProductService) Delete(ctx context.Context, id int64) error {
	key := fmt.Sprintf("product:%d", id)

	err := s.next.Delete(ctx, id)
	if err != nil {
		return err
	}

	s.redisClient.Del(ctx, key)
	return nil
}

func (s *cachedProductService) Create(ctx context.Context, product *domain.Product) (int64, error) {
	return s.next.Create(ctx, product)
}

func (s *cachedProductService) FindByID(ctx context.Context, id int64) (*domain.Product, error) {
	key := fmt.Sprintf("product:%d", id)

	val, err := s.redisClient.Get(ctx, key).Result()
	if err == nil {
		var product domain.Product
		if err := json.Unmarshal([]byte(val), &product); err != nil {
			return &product, nil
		}
	}

	product, err := s.next.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(product); err != nil {
		s.redisClient.Set(ctx, key, data, s.cacheTTL)
	}

	return product, nil
}

func (s *cachedProductService) List(ctx context.Context, limit, offset int64, search string) ([]domain.Product, int64, error) {
	return s.next.List(ctx, limit, offset, search)
}

func (s *cachedProductService) DecreaseStock(ctx context.Context, id, quantity int64) (string, error) {
	res, err := s.next.DecreaseStock(ctx, id, quantity)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("product:%d", id)
	s.redisClient.Del(ctx, key)
	return res, nil
}

func (s *cachedProductService) ReturnStock(ctx context.Context, event *generalDomain.OrderCancelledEvent) error {
	return s.next.ReturnStock(ctx, event)
}

func NewCachedProductService(next ProductService, redisClient *redis.Client) ProductService {
	return &cachedProductService{
		next:        next,
		redisClient: redisClient,
		cacheTTL:    time.Minute * 10,
	}
}
