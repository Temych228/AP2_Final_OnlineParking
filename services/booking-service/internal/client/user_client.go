package client

import (
	"context"

	userv1 "github.com/Temych228/ap2_protos_go_final/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserLookup interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
}

type UserClient struct {
	client userv1.UserServiceClient
}

func NewUserClient(conn *grpc.ClientConn) *UserClient {
	return &UserClient{client: userv1.NewUserServiceClient(conn)}
}

func (c *UserClient) GetUserEmail(ctx context.Context, userID string) (string, error) {
	resp, err := c.client.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
	if err != nil {
		return "", err
	}

	if resp.GetUser() == nil {
		return "", status.Error(codes.NotFound, "user not found")
	}

	return resp.GetUser().GetEmail(), nil
}
