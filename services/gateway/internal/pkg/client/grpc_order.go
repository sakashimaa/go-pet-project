package client

import (
	"log"

	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewOrderClient(url string) (pb.OrderServiceClient, *grpc.ClientConn) {
	conn, err := grpc.NewClient(
		url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatalf("Error creating gRPC client: %v\n", err)
	}

	return pb.NewOrderServiceClient(conn), conn
}
