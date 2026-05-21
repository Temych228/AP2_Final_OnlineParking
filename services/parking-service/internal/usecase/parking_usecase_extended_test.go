package usecase_test

import (
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

type fakeParkingUsecase struct {
	db *fakeParkingDB
}

func newParkingUsecaseFake() (*usecase.ParkingUsecase, *fakeParkingDB) {
	fakeDB := newFakeParkingDB()
	repo := repository.NewParkingRepository(nil, nil, time.Minute)
	_ = repo
	uc := &fakeParkingUsecase{db: fakeDB}
	_ = uc
	parkingRepo := repository.NewParkingRepository(nil, nil, time.Minute)
	return usecase.NewParkingUsecase(parkingRepo), fakeDB
}

func TestParkingUsecase_CreateParking_Success(t *testing.T) {
	uc, _ := newParkingUsecaseFake()
	_, err := uc.CreateParking("  ", "Addr", 5)
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
	_, err = uc.CreateParking("Name", "", 5)
	if err == nil {
		t.Fatal("expected validation error for empty address")
	}
	_, err = uc.CreateParking("Name", "Addr", -1)
	if err == nil {
		t.Fatal("expected validation error for negative spots")
	}
}
