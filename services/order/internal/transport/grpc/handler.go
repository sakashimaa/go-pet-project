package grpc

import (
	"context"

	"github.com/sakashimaa/go-pet-project/order/internal/service"
	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

type OrderHandler struct {
	pb.UnimplementedOrderServiceServer
	service service.OrderService
	logger  *zap.Logger
}

func NewOrderHandler(service service.OrderService, logger *zap.Logger) *OrderHandler {
	return &OrderHandler{service: service, logger: logger}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	res, err := h.service.CreateOrder(ctx, req)

	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"create order failed",
			zap.String("method", "CreateOrder"),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	return &pb.CreateOrderResponse{OrderId: res.OrderId}, nil
}
