package usecase_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

func newParkingUsecaseWithoutDB() *usecase.ParkingUsecase {
	parkingRepo := repository.NewParkingRepository(nil, nil, time.Minute)
	return usecase.NewParkingUsecase(parkingRepo)
}

func TestCreateParkingValidationErrors(t *testing.T) {
	parkingUC := newParkingUsecaseWithoutDB()

	tests := []struct {
		name       string
		parkName   string
		address    string
		totalSpots int
		wantError  string
	}{
		{
			name:       "empty parking name",
			parkName:   "   ",
			address:    "Astana",
			totalSpots: 10,
			wantError:  "parking name is required",
		},
		{
			name:       "empty address",
			parkName:   "Mega Parking",
			address:    "   ",
			totalSpots: 10,
			wantError:  "parking address is required",
		},
		{
			name:       "zero spots",
			parkName:   "Mega Parking",
			address:    "Astana",
			totalSpots: 0,
			wantError:  "total_spots must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parkingUC.CreateParking(tt.parkName, tt.address, tt.totalSpots)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}
