package client

import (
	"log"

	pb "github.com/sakashimaa/go-pet-project/proto/product"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewProductClient(url string) (pb.ProductServiceClient, *grpc.ClientConn) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error creating gRPC client: %v\n", err)
	}

	return pb.NewProductServiceClient(conn), conn
}
