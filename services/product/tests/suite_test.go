package tests

import (
	"context"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"
	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	repository2 "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/testsuite"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
	"github.com/sakashimaa/go-pet-project/product/internal/service"
	"github.com/stretchr/testify/suite"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.uber.org/zap"
)

type IntegrationTestSuite struct {
	testsuite.BaseSuite

	ProductService       service.ProductService
	CachedProductService service.ProductService
	TestProducer         kafka2.Producer
	OutboxProcessor      *worker.OutboxProcessor
	CacheTTL             time.Duration
	workerCancel         context.CancelFunc

	RedisInternalClient *redis.Client
	RedisContainer      *tcredis.RedisContainer
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.BaseSuite.SetupInfrastructure("../migrations")

	redisContainer, err := tcredis.Run(s.Ctx,
		"redis:7-alpine",
	)
	s.Require().NoError(err)

	connStr, err := redisContainer.ConnectionString(s.Ctx)
	s.Require().NoError(err)

	opts, err := redis.ParseURL(connStr)
	s.Require().NoError(err)

	realRedisClient := redis.NewClient(opts)
	s.RedisInternalClient = realRedisClient
	s.RedisContainer = redisContainer
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.RedisContainer != nil {
		_ = s.RedisContainer.Terminate(s.Ctx)
	}
	s.BaseSuite.TearDownInfrastructure()
}

func (s *IntegrationTestSuite) SetupTest() {
	s.BaseSuite.TruncateTable("products")
	s.BaseSuite.TruncateTable("outbox")

	err := s.RedisInternalClient.FlushAll(s.Ctx).Err()
	s.Require().NoError(err)

	logger := zap.NewNop()
	productRepo := repository.NewProductRepository(s.DbPool, logger)
	outboxRepo := repository2.NewOutboxRepository(s.DbPool, logger)

	s.TestProducer, err = kafka2.NewProducer(s.KafkaBrokers)
	s.Require().NoError(err, "failed to create kafka producer")

	s.ProductService = service.NewProductService(productRepo, outboxRepo, s.DbPool, logger)
	s.CachedProductService = service.NewCachedProductService(s.ProductService, s.RedisInternalClient)
	s.OutboxProcessor = worker.NewOutboxProcessor(s.DbPool, outboxRepo, s.TestProducer, logger)

	workerCtx, cancel := context.WithCancel(s.Ctx)
	s.workerCancel = cancel

	go s.OutboxProcessor.Start(workerCtx)
}

func (s *IntegrationTestSuite) TearDownTest() {
	if s.workerCancel != nil {
		s.workerCancel()
	}
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
