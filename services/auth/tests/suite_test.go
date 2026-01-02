package tests

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/sakashimaa/go-pet-project/auth/internal/repository"
	"github.com/sakashimaa/go-pet-project/auth/internal/service"
	"github.com/sakashimaa/go-pet-project/auth/pkg/kafka"
	myValidator "github.com/sakashimaa/go-pet-project/auth/pkg/validator"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	kafkaContainer "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

type IntegrationTestSuite struct {
	suite.Suite
	PgContainer    *postgres.PostgresContainer
	KafkaContainer *kafkaContainer.KafkaContainer
	DbPool         *pgxpool.Pool
	KafkaBrokers   []string
	Ctx            context.Context

	AuthService     service.AuthService
	TestProducer    kafka.Producer
	OutboxProcessor *worker.OutboxProcessor
	workerCancel    context.CancelFunc
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.Ctx = context.Background()

	if err := godotenv.Load("../.env"); err != nil {
		log.Println("no .env filed found, relying on system envs")
	}

	var err error
	s.PgContainer, err = postgres.Run(
		s.Ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)

	s.Require().NoError(err)

	connStr, err := s.PgContainer.ConnectionString(s.Ctx, "sslmode=disable")
	s.Require().NoError(err)

	s.KafkaContainer, err = kafkaContainer.Run(
		s.Ctx,
		"confluentinc/cp-kafka:7.5.0",
		kafkaContainer.WithClusterID("test-cluster"),
	)
	s.Require().NoError(err)

	brokers, err := s.KafkaContainer.Brokers(s.Ctx)
	s.Require().NoError(err)
	s.KafkaBrokers = brokers

	cwd, err := os.Getwd()
	s.Require().NoError(err)

	migrationsPath := filepath.Join(cwd, "..", "migrations")

	sourceURL := "file://" + migrationsPath

	log.Printf("🛠 Migrations Path: %s", sourceURL)

	m, err := migrate.New(sourceURL, connStr)
	s.Require().NoError(err)
	s.Require().NoError(m.Up())

	s.DbPool, err = pgxpool.New(s.Ctx, connStr)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.DbPool != nil {
		s.DbPool.Close()
	}
	if s.PgContainer != nil {
		if err := s.PgContainer.Terminate(s.Ctx); err != nil {
			s.T().Fatalf("failed to terminate postgres container: %v", err)
		}
	}
	if s.KafkaContainer != nil {
		if err := s.KafkaContainer.Terminate(s.Ctx); err != nil {
			s.T().Fatalf("failed to terminate kafka container: %v", err)
		}
	}
	if s.workerCancel != nil {
		s.workerCancel()
	}
}

func (s *IntegrationTestSuite) SetupTest() {
	_, err := s.DbPool.Exec(s.Ctx, "TRUNCATE users CASCADE")
	s.Require().NoError(err)

	logger := zap.NewNop()
	userRepo := repository.NewUserRepository(s.DbPool, logger)
	outboxRepo := repository.NewOutboxRepository(s.DbPool, logger)

	s.TestProducer, err = kafka.NewProducer(s.KafkaBrokers)
	s.Require().NoError(err, "failed to create kafka producer")

	validator := myValidator.NewValidator()

	s.AuthService = service.NewAuthService(userRepo, outboxRepo, s.TestProducer, logger, s.DbPool, validator)

	s.OutboxProcessor = worker.NewOutboxProcessor(s.DbPool, outboxRepo, s.TestProducer, logger)

	workerCtx, cancel := context.WithCancel(s.Ctx)
	s.workerCancel = cancel

	go s.OutboxProcessor.Start(workerCtx)
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
