package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sakashimaa/go-pet-project/payment/internal/repository"
	"github.com/sakashimaa/go-pet-project/payment/internal/service"
	"github.com/sakashimaa/go-pet-project/payment/internal/transport/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/db"
	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outbox "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "payment-service")
	if err != nil {
		log.Fatalf("Error init tracer: %v", err)
	}

	pool, err := db.NewPostgresDB(utils.ParseWithFallback("DB_URL", ""))
	if err != nil {
		log.Fatalf("Error creating postgres DB: %v", err)
	}

	loggerCfg := config.LoggerConfig{
		Level: "info",
		Env:   "dev",
	}

	logger, err := config.NewLogger(loggerCfg)
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Fatalf("failed to sync logger: %v", err)
		}
	}()

	mylogger.Info(
		ctx,
		logger,
		"Payment service started!",
	)

	kafkaHost := utils.ParseWithFallback("KAFKA_HOST", "localhost:9092")

	paymentRepo := repository.NewPaymentRepository(pool, logger)
	outboxRepo := outbox.NewOutboxRepository(pool, logger)
	paymentService := service.NewPaymentService(pool, paymentRepo, outboxRepo, logger)

	consumer := kafka.NewConsumer(paymentService, logger)

	kafkaUrl := utils.ParseWithFallback("KAFKA_URL", "localhost:9092")
	kafkaProducer, err := kafka2.NewProducer([]string{kafkaUrl})
	if err != nil {
		log.Fatalf("error creating kafka producer: %v", err)
	}

	outboxProcessor := worker.NewOutboxProcessor(pool, outboxRepo, kafkaProducer, logger)

	go outboxProcessor.Start(ctx)

	consumer.Start(ctx, []string{kafkaHost})

	<-ctx.Done()

	shutdownCtx, exit := context.WithTimeout(ctx, 5*time.Second)
	defer exit()

	if err := tp.Shutdown(shutdownCtx); err != nil {
		mylogger.Error(
			shutdownCtx,
			logger,
			"Error shutting down telemetry",
		)
	} else {
		mylogger.Error(
			shutdownCtx,
			logger,
			"Telemetry down correctly",
		)
	}

	pool.Close()
	mylogger.Info(shutdownCtx, logger, "Pool down correctly")
}
