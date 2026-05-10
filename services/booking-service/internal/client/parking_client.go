package client

import (
	"context"

	parkingv1 "github.com/Temych228/ap2_protos_go_final/parking/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ParkingLookup interface {
	GetParkingLot(ctx context.Context, parkingLotID string) error
	GetParkingSpotParkingLotID(ctx context.Context, parkingSpotID string) (string, error)
}

type ParkingClient struct {
	client parkingv1.ParkingServiceClient
}

func NewParkingClient(conn *grpc.ClientConn) *ParkingClient {
	return &ParkingClient{client: parkingv1.NewParkingServiceClient(conn)}
}

func (c *ParkingClient) GetParkingLot(ctx context.Context, parkingLotID string) error {
	resp, err := c.client.GetParkingLot(ctx, &parkingv1.GetParkingLotRequest{ParkingLotId: parkingLotID})
	if err != nil {
		return err
	}

	if resp.GetParkingLot() == nil {
		return status.Error(codes.NotFound, "parking lot not found")
	}

	return nil
}

func (c *ParkingClient) GetParkingSpotParkingLotID(ctx context.Context, parkingSpotID string) (string, error) {
	resp, err := c.client.GetParkingSpot(ctx, &parkingv1.GetParkingSpotRequest{ParkingSpotId: parkingSpotID})
	if err != nil {
		return "", err
	}

	spot := resp.GetParkingSpot()
	if spot == nil {
		return "", status.Error(codes.NotFound, "parking spot not found")
	}

	return spot.GetParkingLotId(), nil
}
