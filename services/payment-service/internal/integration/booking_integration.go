package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BookingIntegration struct {
	baseURL string
	client  *http.Client
}

func NewBookingIntegration(baseURL string) *BookingIntegration {
	return &BookingIntegration{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type BookingInfo struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ParkingID int64     `json:"parking_id"`
	SpotID    int64     `json:"spot_id"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (b *BookingIntegration) GetBooking(ctx context.Context, bookingID string) (*BookingInfo, error) {
	url := fmt.Sprintf("%s/bookings/%s", b.baseURL, bookingID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("booking-service returned status: %d", resp.StatusCode)
	}

	var booking BookingInfo
	if err := json.NewDecoder(resp.Body).Decode(&booking); err != nil {
		return nil, err
	}

	return &booking, nil
}

func (b *BookingIntegration) ConfirmBooking(ctx context.Context, bookingID string) error {
	url := fmt.Sprintf("%s/bookings/%s/confirm", b.baseURL, bookingID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("booking confirmation failed, status: %d", resp.StatusCode)
	}

	return nil
}
