package testsuite

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BaseSuite struct {
	suite.Suite
	PgContainer    *postgres.PostgresContainer
	KafkaContainer *kafka.KafkaContainer
	DbPool         *pgxpool.Pool
	KafkaBrokers   []string
	Ctx            context.Context
}

func (s *BaseSuite) SetupInfrastructure(migrationsRelPath string) {
	s.Ctx = context.Background()

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

	s.KafkaContainer, err = kafka.Run(
		s.Ctx,
		"confluentinc/cp-kafka:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	s.Require().NoError(err)

	s.KafkaBrokers, err = s.KafkaContainer.Brokers(s.Ctx)
	s.Require().NoError(err)

	absPath, err := filepath.Abs(migrationsRelPath)
	s.Require().NoError(err)

	sourceURL := "file://" + absPath
	log.Printf("🔨 Running migrations from: %s", sourceURL)

	m, err := migrate.New(sourceURL, connStr)
	s.Require().NoError(err)
	s.Require().NoError(m.Up())

	s.DbPool, err = pgxpool.New(s.Ctx, connStr)
	s.Require().NoError(err)
}

func (s *BaseSuite) TearDownInfrastructure() {
	if s.DbPool != nil {
		s.DbPool.Close()
	}
	if s.PgContainer != nil {
		if err := s.PgContainer.Terminate(s.Ctx); err != nil {
			log.Printf("Failed to terminate postgres container: %v", err)
		}
	}
	if s.KafkaContainer != nil {
		if err := s.KafkaContainer.Terminate(s.Ctx); err != nil {
			log.Printf("Failed to terminate kafka container: %v", err)
		}
	}
}

func (s *BaseSuite) TruncateTable(tableName string) {
	_, err := s.DbPool.Exec(s.Ctx, fmt.Sprintf("TRUNCATE %s CASCADE", tableName))
	s.Require().NoError(err)
}
