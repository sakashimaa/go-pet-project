package main

import (
	"context"
	"log"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sakashimaa/go-pet-project/order/internal/repository"
	"github.com/sakashimaa/go-pet-project/order/internal/service"
	"github.com/sakashimaa/go-pet-project/order/internal/transport/grpc"
	"github.com/sakashimaa/go-pet-project/order/internal/transport/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/db"
	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	repository2 "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("error loading .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "order-service")
	if err != nil {
		log.Fatalf("failed to init tracer: %v", err)
	}

	pool, err := db.NewPostgresDB(utils.ParseWithFallback("DB_URL", ""))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	loggerCfg := config.LoggerConfig{
		Level: "Info",
		Env:   "dev",
	}
	logger, err := config.NewLogger(loggerCfg)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Fatalf("failed to sync logger: %v", err)
		}
	}()

	orderRepo := repository.NewOrderRepository(pool, logger)
	outboxRepo := repository2.NewOutboxRepository(pool, logger)
	orderService := service.NewOrderService(pool, logger, orderRepo, outboxRepo)
	orderHandler := grpc.NewOrderHandler(orderService, logger)

	kafkaUrl := utils.ParseWithFallback("KAFKA_URL", "localhost:9092")
	kafkaProducer, err := kafka2.NewProducer([]string{kafkaUrl})
	if err != nil {
		log.Fatalf("error creating kafka producer: %v", err)
	}

	outboxProcessor := worker.NewOutboxProcessor(pool, outboxRepo, kafkaProducer, logger)

	go outboxProcessor.Start(ctx)

	kafkaHost := utils.ParseWithFallback("KAFKA_HOST", "localhost:9092")

	consumer := kafka.NewConsumer(orderService, logger)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Error listening on :50053 %v", err)
	}

	s := googleGrpc.NewServer()
	pb.RegisterOrderServiceServer(s, orderHandler)

	go func() {
		log.Println("gRPC server listening on 50053 🔥")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Error serving gRPC: %v", err)
		}
	}()

	consumer.Start(ctx, []string{kafkaHost})

	<-ctx.Done()

	shutdownCtx, exit := context.WithTimeout(ctx, time.Second*5)
	defer exit()

	mylogger.Info(
		shutdownCtx,
		logger,
		"Shutting down order server",
	)

	s.GracefulStop()
	log.Println("✅ gRPC service stopped")

	if err := tp.Shutdown(shutdownCtx); err != nil {
		mylogger.Warn(
			shutdownCtx,
			logger,
			"Failed to shut down telemetry",
			zap.Error(err),
		)
	} else {
		mylogger.Info(
			shutdownCtx,
			logger,
			"Successfully down telemetry",
		)
	}

	pool.Close()
}
