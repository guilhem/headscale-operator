package utils

import (
	"context"

	headscaleapiv1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewHeadscaleServiceClient(ctx context.Context, address string) (headscaleapiv1.HeadscaleServiceClient, error) {
	grpcOptions := []grpc.DialOption{
		grpc.WithBlock(),
	}

	grpcOptions = append(grpcOptions,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	// tlsConfig := &tls.Config{
	// 	// turn of gosec as we are intentionally setting
	// 	// insecure.
	// 	//nolint:gosec
	// 	InsecureSkipVerify: true,
	// }

	// grpcOptions = append(grpcOptions,
	// 	grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	// )

	conn, err := grpc.DialContext(ctx, address, grpcOptions...)
	if err != nil {
		return nil, err
	}

	return headscaleapiv1.NewHeadscaleServiceClient(conn), nil
}
