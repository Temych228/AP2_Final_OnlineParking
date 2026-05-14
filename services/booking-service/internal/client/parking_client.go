package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	parkingv1 "github.com/Temych228/ap2_protos_go_final/parking/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ParkingLookup interface {
	GetParkingLot(ctx context.Context, parkingLotID string) error
	GetParkingSpotParkingLotID(ctx context.Context, parkingSpotID string) (string, error)
	ListParkingSpots(ctx context.Context, parkingLotID string) ([]SpotInfo, error)
	ReserveSpot(ctx context.Context, spotID string) error
	ReleaseSpot(ctx context.Context, spotID string) error
	UpdateSpotStatus(ctx context.Context, spotID string, status string) error
}

type SpotInfo struct {
	ID        int64  `json:"id"`
	ParkingID int64  `json:"parking_id"`
	Number    string `json:"number"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}

type ParkingClient struct {
	client      parkingv1.ParkingServiceClient
	httpBaseURL string
	httpClient  *http.Client
}

func NewParkingClient(conn *grpc.ClientConn, httpBaseURL string) *ParkingClient {
	return &ParkingClient{
		client:      parkingv1.NewParkingServiceClient(conn),
		httpBaseURL: strings.TrimRight(strings.TrimSpace(httpBaseURL), "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
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

func (c *ParkingClient) ListParkingSpots(ctx context.Context, parkingLotID string) ([]SpotInfo, error) {
	if c.httpBaseURL == "" {
		return nil, fmt.Errorf("parking http url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.httpBaseURL+"/parkings/"+url.PathEscape(strings.TrimSpace(parkingLotID))+"/spots", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("parking-service returned status: %d", resp.StatusCode)
	}

	var spots []SpotInfo
	if err := json.NewDecoder(resp.Body).Decode(&spots); err != nil {
		return nil, err
	}

	return spots, nil
}

func (c *ParkingClient) ReserveSpot(ctx context.Context, spotID string) error {
	return c.postSpotAction(ctx, spotID, "reserve", nil)
}

func (c *ParkingClient) ReleaseSpot(ctx context.Context, spotID string) error {
	return c.postSpotAction(ctx, spotID, "release", nil)
}

func (c *ParkingClient) UpdateSpotStatus(ctx context.Context, spotID string, statusValue string) error {
	return c.postSpotAction(ctx, spotID, "status", map[string]string{"status": strings.TrimSpace(statusValue)})
}

func (c *ParkingClient) postSpotAction(ctx context.Context, spotID string, action string, payload any) error {
	if c.httpBaseURL == "" {
		return fmt.Errorf("parking http url is required")
	}

	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.httpBaseURL+"/spots/"+url.PathEscape(strings.TrimSpace(spotID))+"/"+action, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("parking-service returned status: %d", resp.StatusCode)
	}

	return nil
}
