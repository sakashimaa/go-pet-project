package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/joho/godotenv"
	"github.com/sakashimaa/go-pet-project/gateway/internal/pkg/client"
	"github.com/sakashimaa/go-pet-project/gateway/internal/transport/http"
	"github.com/sakashimaa/go-pet-project/gateway/internal/transport/http/handler"
	"github.com/sakashimaa/go-pet-project/pkg/config"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env not found: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := utils.InitTracer(ctx, "gateway-service")
	if err != nil {
		log.Fatalf("Failed to init trace: %v", err)
	}

	port := utils.ParseWithFallback("PORT", ":3000")
	authUrl := utils.ParseWithFallback("AUTH_RPC_URL", "localhost:50051")
	productUrl := utils.ParseWithFallback("PRODUCT_RPC_URL", "localhost:50052")
	orderUrl := utils.ParseWithFallback("ORDER_RPC_URL", "localhost:50053")

	app := fiber.New()

	app.Use(otelfiber.Middleware())

	app.Use(limiter.New(limiter.Config{
		Max:        20,
		Expiration: 5 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Try again later.",
			})
		},
	}))

	authServiceClient, authConn := client.NewAuthClient(authUrl)
	defer func() {
		if err := authConn.Close(); err != nil {
			log.Fatalf("Error closing auth connection: %v", err)
		}
	}()

	productServiceClient, productConn := client.NewProductClient(productUrl)
	defer func() {
		if err := productConn.Close(); err != nil {
			log.Fatalf("Error closing product connection: %v", err)
		}
	}()

	orderServiceClient, orderConn := client.NewOrderClient(orderUrl)
	defer func() {
		if err := orderConn.Close(); err != nil {
			log.Fatalf("Error closing order connection: %v", err)
		}
	}()

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

	logger.Info("Gateway service started!")

	handlers := &http.Handlers{
		Auth:    handler.NewAuthHandler(authServiceClient, logger),
		Product: handler.NewProductHandler(productServiceClient, logger),
		Order:   handler.NewOrderHandler(orderServiceClient, logger),
	}

	http.RegisterRoutes(app, handlers, authServiceClient)

	go func() {
		log.Println("HTTP Service listening on: " + port)
		if err := app.Listen(port); err != nil {
			log.Fatalf("Error listening on HTTP port %v: %v\n", port, err)
		}
	}()

	<-ctx.Done()

	log.Println("Shutting down gracefully...")
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownContext); err != nil {
		log.Printf("Error shutting down HTTP app: %v\n", err)
	} else {
		log.Println("HTTP App stopped gracefully")
	}

	if err := tp.Shutdown(shutdownContext); err != nil {
		log.Printf("Error shutting down telemetry: %v\n", err)
	} else {
		log.Println("Telemetry stopped correctly")
	}
}
