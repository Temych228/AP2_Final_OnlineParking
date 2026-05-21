package usecase_test

import (
	"strings"
	"testing"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

func newParkingUC() *usecase.ParkingUsecase {
	return usecase.NewParkingUsecase(newFakeParkingDB())
}

func TestCreateParkingValidationErrors(t *testing.T) {
	uc := newParkingUC()

	tests := []struct {
		name       string
		parkName   string
		address    string
		totalSpots int
		wantError  string
	}{
		{"empty name", "   ", "Astana", 10, "parking name is required"},
		{"empty address", "Mega", "   ", 10, "parking address is required"},
		{"zero spots", "Mega", "Astana", 0, "total_spots must be greater than zero"},
		{"negative spots", "Mega", "Astana", -5, "total_spots must be greater than zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.CreateParking(tt.parkName, tt.address, tt.totalSpots)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestParkingUsecase_CRUD(t *testing.T) {
	uc := newParkingUC()

	p, err := uc.CreateParking("Astana Mall", "Kerey St 10", 50)
	if err != nil {
		t.Fatalf("CreateParking: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := uc.GetParking(p.ID)
	if err != nil {
		t.Fatalf("GetParking: %v", err)
	}
	if got.Name != "Astana Mall" {
		t.Fatalf("unexpected name: %s", got.Name)
	}

	all, err := uc.GetAllParkings()
	if err != nil {
		t.Fatalf("GetAllParkings: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 parking, got %d", len(all))
	}

	if err := uc.DeleteParking(p.ID); err != nil {
		t.Fatalf("DeleteParking: %v", err)
	}

	all2, _ := uc.GetAllParkings()
	if len(all2) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(all2))
	}
}

func TestParkingUsecase_GetMissing(t *testing.T) {
	uc := newParkingUC()
	if _, err := uc.GetParking(999); err == nil {
		t.Fatal("expected error for missing parking")
	}
}

func TestParkingUsecase_DeleteMissing(t *testing.T) {
	uc := newParkingUC()
	if err := uc.DeleteParking(999); err == nil {
		t.Fatal("expected error for missing parking")
	}
}

func TestParkingUsecase_MultipleCreate(t *testing.T) {
	uc := newParkingUC()

	names := []string{"Lot A", "Lot B", "Lot C"}
	for _, n := range names {
		if _, err := uc.CreateParking(n, "Addr", 20); err != nil {
			t.Fatalf("CreateParking %s: %v", n, err)
		}
	}

	all, err := uc.GetAllParkings()
	if err != nil {
		t.Fatalf("GetAllParkings: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}
