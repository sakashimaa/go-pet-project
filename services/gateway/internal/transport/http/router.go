package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sakashimaa/go-pet-project/gateway/internal/transport/http/handler"
	"github.com/sakashimaa/go-pet-project/gateway/middleware"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
)

type Handlers struct {
	Auth    *handler.AuthHandler
	Product *handler.ProductHandler
	Order   *handler.OrderHandler
}

func RegisterRoutes(app *fiber.App, h *Handlers, authClient pb.AuthServiceClient) {
	authGroup := app.Group("/auth")

	authGroup.Post("/register", h.Auth.Register)
	authGroup.Post("/refresh", h.Auth.Refresh)
	authGroup.Post("/login", h.Auth.Login)
	authGroup.Post("/reset-password", h.Auth.ResetPassword)
	authGroup.Post("/forgot-password", h.Auth.ForgotPassword)
	authGroup.Get("/activate", h.Auth.Activate)
	authGroup.Post("/logout", h.Auth.Logout)

	api := app.Group("/api", middleware.NewAuthMiddleware(authClient), middleware.NewIsActivatedMiddleware())
	api.Get("/me", h.Auth.GetMe)

	product := api.Group("/products")
	product.Post("", h.Product.Create)
	product.Post("/decrease-stock/:id", h.Product.DecreaseStock)
	product.Delete("/:id", h.Product.DeleteProduct)
	product.Get("/:id", h.Product.FindByID)
	product.Get("", h.Product.ListProducts)

	order := api.Group("/orders")
	order.Post("", h.Order.Create)
}
