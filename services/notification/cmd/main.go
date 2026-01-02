package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sakashimaa/go-pet-project/notification/internal/infrastructure/email"
	"github.com/sakashimaa/go-pet-project/notification/internal/service"
	"github.com/sakashimaa/go-pet-project/notification/transport/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/db"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("❌ error loading env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "notification-service")
	if err != nil {
		log.Fatalf("Error starting telemetry: %v", err)
	}

	cfg := config.LoggerConfig{
		Level: "info",
		Env:   "dev",
	}

	logger, err := config.NewLogger(cfg)
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Fatalf("error syncing logger: %v", err)
		}
	}()

	pool, err := db.NewPostgresDB(utils.ParseWithFallback("DB_URL", ""))
	if err != nil {
		log.Fatalf("error creating postgres db: %v", err)
	}

	kafkaHost := utils.ParseWithFallback("KAFKA_HOST", "localhost:9092")
	emailSender := email.NewSMTPSender(logger)
	notificationService := service.NewNotificationService(emailSender, logger, pool)

	consumer := kafka.NewConsumer(notificationService, logger)

	consumer.Start(ctx, []string{kafkaHost})

	<-ctx.Done()

	shutdownCtx, exit := context.WithTimeout(context.Background(), 5*time.Second)
	defer exit()

	if err := tp.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error closing telemetry: %v\n", err)
	} else {
		log.Printf("Closed telemetry successfully")
	}

	pool.Close()
	log.Println("✅ Postgres pool closed")
}
