package grpc

import (
	"context"

	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/service"
	pb "github.com/sakashimaa/go-pet-project/proto/product"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

type ProductHandler struct {
	pb.UnimplementedProductServiceServer
	service service.ProductService
	logger  *zap.Logger
}

func NewProductHandler(service service.ProductService, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{service: service, logger: logger}
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	err := h.service.Delete(ctx, req.Id)
	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"delete product failed",
			zap.String("method", "DeleteProduct"),
			zap.Int64("product_id", req.Id),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	return &pb.DeleteProductResponse{
		Success: true,
	}, nil
}

func (h *ProductHandler) DecreaseStock(ctx context.Context, req *pb.DecreaseStockRequest) (*pb.DecreaseStockResponse, error) {
	message, err := h.service.DecreaseStock(ctx, req.ProductId, req.Quantity)
	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"decrease stock failed",
			zap.String("method", "DecreaseStock"),
			zap.Int64("product_id", req.ProductId),
			zap.Int64("quantity", req.Quantity),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	return &pb.DecreaseStockResponse{
		Message: message,
		Success: true,
	}, nil
}

func (h *ProductHandler) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	list, quantity, err := h.service.List(ctx, req.Limit, req.Offset, req.Search)
	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"list products failed",
			zap.String("method", "ListProducts"),
			zap.Int64("offset", req.Offset),
			zap.Int64("limit", req.Limit),
			zap.String("search", req.Search),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	responseList := make([]*pb.Product, 0, len(list))

	for _, p := range list {
		protoProduct := &pb.Product{
			Id:            p.ID,
			Name:          p.Name,
			Description:   p.Description,
			Price:         p.Price,
			StockQuantity: p.StockQuantity,
			ImageUrl:      p.ImageUrl,
			Category:      p.Category,
		}

		responseList = append(responseList, protoProduct)
	}

	return &pb.ListProductsResponse{
		Products:   responseList,
		TotalCount: quantity,
	}, nil
}

func (h *ProductHandler) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	res, err := h.service.FindByID(ctx, req.Id)
	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"get product failed",
			zap.String("method", "GetProduct"),
			zap.Int64("product_id", req.Id),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	productProto := &pb.Product{
		Id:            res.ID,
		Name:          res.Name,
		Description:   res.Description,
		Price:         res.Price,
		StockQuantity: res.StockQuantity,
		ImageUrl:      res.ImageUrl,
		Category:      res.Category,
	}

	return &pb.GetProductResponse{
		Product: productProto,
	}, nil
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	product := domain.Product{
		Name:          req.Name,
		Description:   req.Description,
		Price:         req.Price,
		StockQuantity: req.StockQuantity,
		Category:      req.Category,
	}

	res, err := h.service.Create(ctx, &product)
	if err != nil {
		code := mapErrorCode(err)

		h.logger.Error(
			"create product failed",
			zap.String("method", "CreateProduct"),
			zap.String("name", req.Name),
			zap.String("description", req.Description),
			zap.Int64("price", req.Price),
			zap.Int64("stock_quantity", req.StockQuantity),
			zap.String("category", req.Category),
			zap.String("status_code", code.String()),
			zap.Error(err),
		)

		return nil, status.Error(code, code.String())
	}

	return &pb.CreateProductResponse{
		Id: res,
	}, nil
}
