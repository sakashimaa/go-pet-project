package tests

import (
	"context"
	"testing"

	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	repository2 "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/testsuite"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
	"github.com/sakashimaa/go-pet-project/product/internal/service"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type IntegrationTestSuite struct {
	testsuite.BaseSuite

	ProductService  service.ProductService
	TestProducer    kafka2.Producer
	OutboxProcessor *worker.OutboxProcessor
	workerCancel    context.CancelFunc
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.BaseSuite.SetupInfrastructure("../migrations")
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.BaseSuite.TearDownInfrastructure()
}

func (s *IntegrationTestSuite) SetupTest() {
	s.BaseSuite.TruncateTable("products")
	s.BaseSuite.TruncateTable("outbox")

	logger := zap.NewNop()
	productRepo := repository.NewProductRepository(s.DbPool, logger)
	outboxRepo := repository2.NewOutboxRepository(s.DbPool, logger)

	var err error
	s.TestProducer, err = kafka2.NewProducer(s.KafkaBrokers)
	s.Require().NoError(err, "failed to create kafka producer")

	s.ProductService = service.NewProductService(productRepo, outboxRepo, s.DbPool, logger)
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
