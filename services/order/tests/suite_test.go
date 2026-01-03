package tests

import (
	"context"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sakashimaa/go-pet-project/order/internal/repository"
	"github.com/sakashimaa/go-pet-project/order/internal/service"
	"github.com/sakashimaa/go-pet-project/pkg/domain"
	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	outboxRepository "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/testsuite"
	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type IntegrationTestSuite struct {
	testsuite.BaseSuite

	OrderService    service.OrderService
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
	s.BaseSuite.TruncateTable("orders")
	s.BaseSuite.TruncateTable("users")
	s.BaseSuite.TruncateTable("order_items")
	s.BaseSuite.TruncateTable("outbox")

	logger := zap.NewNop()
	orderRepo := repository.NewOrderRepository(s.DbPool, logger)
	outboxRepo := outboxRepository.NewOutboxRepository(s.DbPool, logger)

	var err error
	s.TestProducer, err = kafka2.NewProducer(s.KafkaBrokers)
	s.Require().NoError(err, "failed to create kafka producer")

	s.OrderService = service.NewOrderService(s.DbPool, logger, orderRepo, outboxRepo)

	s.OutboxProcessor = worker.NewOutboxProcessor(s.DbPool, outboxRepo, s.TestProducer, logger)

	workerCtx, cancel := context.WithCancel(s.Ctx)
	s.workerCancel = cancel

	go s.OutboxProcessor.Start(workerCtx)
}

func (s *IntegrationTestSuite) seedData(id int64, email string) {
	query := `
		INSERT INTO users (id, email)
		VALUES ($1, $2) ON CONFLICT DO NOTHING
	`

	_, err := s.DbPool.Exec(s.Ctx, query, id, email)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownTest() {
	if s.workerCancel != nil {
		s.workerCancel()
	}
}

func (s *IntegrationTestSuite) createOrder(userId int64) *pb.CreateOrderResponse {
	domainItems := []domain.OrderItem{
		{
			ProductID: 1,
			Name:      "Kuronami No Yaiba",
			Price:     5350,
			Quantity:  1,
		},
	}

	pbItems := make([]*pb.OrderItem, len(domainItems))
	for i, item := range domainItems {
		pbItems[i] = item.ToPB()
	}

	resp, err := s.OrderService.CreateOrder(s.Ctx, &pb.CreateOrderRequest{
		UserId: 999,
		Items:  pbItems,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	return resp
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
