package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/db"
	kafka2 "github.com/sakashimaa/go-pet-project/pkg/kafka"
	outbox "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
	"github.com/sakashimaa/go-pet-project/product/internal/service"
	"github.com/sakashimaa/go-pet-project/product/internal/transport/grpc"
	productKafka "github.com/sakashimaa/go-pet-project/product/internal/transport/kafka"
	pb "github.com/sakashimaa/go-pet-project/proto/product"
	googleGrpc "google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("error loading .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "product-service")
	if err != nil {
		log.Fatalf("Error init tracer: %v", err)
	}

	pool, err := db.NewPostgresDB(os.Getenv("DB_URL"))
	if err != nil {
		log.Fatalf("Error creating new postgres DB: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

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

	logger.Info("product service started!")

	productRepository := repository.NewProductRepository(pool, logger)
	outboxRepository := outbox.NewOutboxRepository(pool, logger)
	productService := service.NewProductService(productRepository, outboxRepository, pool, logger)
	cachedProductService := service.NewCachedProductService(productService, rdb)
	productHandler := grpc.NewProductHandler(cachedProductService, logger)

	kafkaUrl := utils.ParseWithFallback("KAFKA_URL", "localhost:9092")
	kafkaProducer, err := kafka2.NewProducer([]string{kafkaUrl})
	if err != nil {
		log.Fatalf("error creating kafka producer: %v", err)
	}

	kafkaHost := utils.ParseWithFallback("KAFKA_HOST", "localhost:9092")

	consumer := productKafka.NewConsumer(productService, logger)

	outboxProcessor := worker.NewOutboxProcessor(pool, outboxRepository, kafkaProducer, logger)

	go outboxProcessor.Start(ctx)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Error listening on :50052 %v", err)
	}

	s := googleGrpc.NewServer()
	pb.RegisterProductServiceServer(s, productHandler)

	go func() {
		log.Println("gRPC server listening on 50052 🔥")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Error serving gRPC: %v", err)
		}
	}()

	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("Product Service is alive!")
	})

	port := utils.ParseWithFallback("PORT", ":3002")

	go func() {
		log.Println("HTTP Product service listening on port: " + port)
		if err := app.Listen(port); err != nil {
			log.Fatalf("Error listening HTTP on port %v: %v", port, err)
		}
	}()

	consumer.Start(ctx, []string{kafkaHost})

	<-ctx.Done()

	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.GracefulStop()
	log.Println("✅ gRPC service stopped")

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	} else {
		log.Println("Stopped HTTP server successfully")
	}

	pool.Close()
	log.Println("Closed db pool successfully")

	if err := tp.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error stopping telemetry: %v\n", err)
	} else {
		log.Println("Telemetry closed correctly")
	}
}
