package client

import (
	"log"

	pb "github.com/sakashimaa/go-pet-project/proto/auth"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewAuthClient(url string) (pb.AuthServiceClient, *grpc.ClientConn) {
	conn, err := grpc.NewClient(
		url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatalf("Error creating gRPC client: %v\n", err)
	}

	return pb.NewAuthServiceClient(conn), conn
}
