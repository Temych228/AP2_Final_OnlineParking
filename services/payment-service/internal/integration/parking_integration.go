package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ParkingIntegration struct {
	baseURL string
	client  *http.Client
}

func NewParkingIntegration(baseURL string) *ParkingIntegration {
	return &ParkingIntegration{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type CalculatePriceResponse struct {
	ParkingID    int64   `json:"parking_id"`
	Hours        float64 `json:"hours"`
	PricePerHour float64 `json:"price_per_hour"`
	TotalPrice   float64 `json:"total_price"`
}

func (p *ParkingIntegration) CalculatePrice(ctx context.Context, parkingID int64, hours float64) (float64, error) {
	url := fmt.Sprintf("%s/tariffs/%d/calculate?hours=%f", p.baseURL, parkingID, hours)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("parking-service returned status: %d", resp.StatusCode)
	}

	var result CalculatePriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.TotalPrice, nil
}
