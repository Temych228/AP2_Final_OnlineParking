package usecase_test

import (
	"strings"
	"testing"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

func newSpotUC() (*usecase.SpotUsecase, *fakeParkingDB, *fakeSpotDB) {
	pDB := newFakeParkingDB()
	sDB := newFakeSpotDB()
	return usecase.NewSpotUsecase(sDB, pDB), pDB, sDB
}

func TestCreateSpotValidationErrors(t *testing.T) {
	uc, _, _ := newSpotUC()

	tests := []struct {
		name      string
		parkingID int64
		number    string
		wantError string
	}{
		{"missing parking id", 0, "A1", "parking_id is required"},
		{"empty spot number", 1, "   ", "spot number is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.CreateSpot(tt.parkingID, tt.number)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestSpotUsecase_CreateAndGet(t *testing.T) {
	uc, pDB, _ := newSpotUC()

	_ = pDB.Create(&domain.Parking{Name: "P1", Address: "A1", TotalSpots: 5})

	spot, err := uc.CreateSpot(1, "A-01")
	if err != nil {
		t.Fatalf("CreateSpot: %v", err)
	}
	if spot.ID == 0 {
		t.Fatal("expected non-zero spot ID")
	}
	if spot.Status != domain.StatusAvailable {
		t.Fatalf("expected available, got %s", spot.Status)
	}

	got, err := uc.GetSpot(spot.ID)
	if err != nil {
		t.Fatalf("GetSpot: %v", err)
	}
	if got.Number != "A-01" {
		t.Fatalf("unexpected number: %s", got.Number)
	}
}

func TestSpotUsecase_LimitReached(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 2})

	if _, err := uc.CreateSpot(1, "S1"); err != nil {
		t.Fatalf("first spot: %v", err)
	}
	if _, err := uc.CreateSpot(1, "S2"); err != nil {
		t.Fatalf("second spot: %v", err)
	}
	if _, err := uc.CreateSpot(1, "S3"); err == nil {
		t.Fatal("expected limit error")
	}
}

func TestSpotUsecase_ReserveAndRelease(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 10})

	spot, _ := uc.CreateSpot(1, "B-01")

	avail, err := uc.CheckAvailability(spot.ID)
	if err != nil || !avail {
		t.Fatalf("expected available: err=%v avail=%v", err, avail)
	}

	if err := uc.ReserveSpot(spot.ID); err != nil {
		t.Fatalf("ReserveSpot: %v", err)
	}

	avail, _ = uc.CheckAvailability(spot.ID)
	if avail {
		t.Fatal("expected not available after reserve")
	}

	if err := uc.ReserveSpot(spot.ID); err == nil {
		t.Fatal("expected error on double reserve")
	}

	if err := uc.ReleaseSpot(spot.ID); err != nil {
		t.Fatalf("ReleaseSpot: %v", err)
	}

	avail, _ = uc.CheckAvailability(spot.ID)
	if !avail {
		t.Fatal("expected available after release")
	}
}

func TestSpotUsecase_ReleaseNotReserved(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 10})
	spot, _ := uc.CreateSpot(1, "C-01")

	if err := uc.ReleaseSpot(spot.ID); err == nil {
		t.Fatal("expected error releasing non-reserved spot")
	}
}

func TestSpotUsecase_GetByParking(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 10})

	uc.CreateSpot(1, "D-01")
	uc.CreateSpot(1, "D-02")
	uc.CreateSpot(1, "D-03")

	spots, err := uc.GetSpotsByParking(1)
	if err != nil {
		t.Fatalf("GetSpotsByParking: %v", err)
	}
	if len(spots) != 3 {
		t.Fatalf("expected 3 spots, got %d", len(spots))
	}
}

func TestSpotUsecase_Delete(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 10})
	spot, _ := uc.CreateSpot(1, "E-01")

	if err := uc.DeleteSpot(spot.ID); err != nil {
		t.Fatalf("DeleteSpot: %v", err)
	}
	if _, err := uc.GetSpot(spot.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSpotUsecase_UpdateStatus(t *testing.T) {
	uc, pDB, _ := newSpotUC()
	_ = pDB.Create(&domain.Parking{Name: "P", Address: "A", TotalSpots: 10})
	spot, _ := uc.CreateSpot(1, "F-01")

	if err := uc.UpdateSpotStatus(spot.ID, domain.StatusReserved); err != nil {
		t.Fatalf("UpdateSpotStatus: %v", err)
	}
	got, _ := uc.GetSpot(spot.ID)
	if got.Status != domain.StatusReserved {
		t.Fatalf("expected reserved, got %s", got.Status)
	}
}
