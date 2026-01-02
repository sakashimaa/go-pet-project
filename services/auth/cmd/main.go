package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sakashimaa/go-pet-project/auth/internal/repository"
	"github.com/sakashimaa/go-pet-project/auth/internal/service"
	"github.com/sakashimaa/go-pet-project/auth/internal/transport/grpc"
	myValidator "github.com/sakashimaa/go-pet-project/auth/pkg/validator"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/db"
	"github.com/sakashimaa/go-pet-project/pkg/kafka"
	outbox "github.com/sakashimaa/go-pet-project/pkg/outbox/repository"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	googleGrpc "google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, using system envs")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "auth-service")
	if err != nil {
		log.Fatalf("Error init tracer: %v", err)
	}

	pool, err := db.NewPostgresDB(utils.ParseWithFallback("DB_URL", "postgres://user:password@localhost:5432/auth_db?sslmode=disable"))
	if err != nil {
		log.Fatalf("error creating postgres db: %v", err)
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
			log.Fatalf("error syncing logger: %v", err)
		}
	}()

	userRepo := repository.NewUserRepository(pool, logger)
	outboxRepo := outbox.NewOutboxRepository(pool, logger)

	kafkaUrl := os.Getenv("KAFKA_URL")
	if kafkaUrl == "" {
		kafkaUrl = "localhost:9092"
	}
	kafkaProducer, err := kafka.NewProducer([]string{kafkaUrl})
	if err != nil {
		log.Fatalf("error creating kafka producer: %v", err)
	}

	outboxProcessor := worker.NewOutboxProcessor(pool, outboxRepo, kafkaProducer, logger)

	go outboxProcessor.Start(ctx)

	logger.Info("auth service started!")

	validator := myValidator.NewValidator()

	authService := service.NewAuthService(userRepo, outboxRepo, kafkaProducer, logger, pool, validator)
	authHandler := grpc.NewAuthHandler(authService, logger)

	reg := prometheus.NewRegistry()

	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	grpc_prometheus.EnableHandlingTimeHistogram()

	reg.MustRegister(grpc_prometheus.DefaultServerMetrics)

	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			Registry: reg,
		}))
		log.Println("Metrics server is listening on 9091 📈")

		if err := http.ListenAndServe(":9091", nil); err != nil {
			log.Printf("Metrics serving failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("error listening on tcp: %v", err)
	}

	s := googleGrpc.NewServer(
		googleGrpc.StatsHandler(otelgrpc.NewServerHandler()),
		googleGrpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		googleGrpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	pb.RegisterAuthServiceServer(s, authHandler)

	grpc_prometheus.Register(s)

	go func() {
		log.Println("gRPC server listening on 50051 🔥")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Error serving gRPC: %v", err)
		}
	}()

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("Auth Service is alive!")
	})

	port := utils.ParseWithFallback("PORT", ":3001")

	go func() {
		log.Println("HTTP Server listening on port: " + port)
		if err := app.Listen(port); err != nil {
			log.Fatalf("Error listening on HTTP: %v", err)
		}
	}()

	<-ctx.Done()

	time.Sleep(1 * time.Second)

	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.GracefulStop()
	log.Println("✅ gRPC server stopped")

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP: %v\n", err)
	} else {
		log.Printf("HTTP Server stopped")
	}

	if err := kafkaProducer.Close(); err != nil {
		log.Printf("Kafka close error: %v", err)
	} else {
		log.Printf("Kafka producer closed")
	}

	pool.Close()
	log.Println("✅ Postgres pool closed")

	if err := tp.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Error closing telemetry: %v\n", err)
	} else {
		log.Println("✅ Telemetry closed")
	}
}
