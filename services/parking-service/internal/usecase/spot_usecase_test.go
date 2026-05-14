package usecase_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

func newSpotUsecaseWithoutDB() *usecase.SpotUsecase {
	parkingRepo := repository.NewParkingRepository(nil, nil, time.Minute)
	spotRepo := repository.NewSpotRepository(nil)
	return usecase.NewSpotUsecase(spotRepo, parkingRepo)
}

func TestCreateSpotValidationErrors(t *testing.T) {
	spotUC := newSpotUsecaseWithoutDB()

	tests := []struct {
		name      string
		parkingID int64
		number    string
		wantError string
	}{
		{
			name:      "missing parking id",
			parkingID: 0,
			number:    "A1",
			wantError: "parking_id is required",
		},
		{
			name:      "empty spot number",
			parkingID: 1,
			number:    "   ",
			wantError: "spot number is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := spotUC.CreateSpot(tt.parkingID, tt.number)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}
