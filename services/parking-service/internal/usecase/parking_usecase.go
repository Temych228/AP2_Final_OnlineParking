package usecase

import (
	"errors"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
)

type ParkingUsecase struct {
	parkingRepo *repository.ParkingRepository
}

func NewParkingUsecase(parkingRepo *repository.ParkingRepository) *ParkingUsecase {
	return &ParkingUsecase{parkingRepo: parkingRepo}
}

func (u *ParkingUsecase) CreateParking(name, address string, totalSpots int) (*domain.Parking, error) {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)

	if name == "" {
		return nil, errors.New("parking name is required")
	}

	if address == "" {
		return nil, errors.New("parking address is required")
	}

	if totalSpots <= 0 {
		return nil, errors.New("total_spots must be greater than zero")
	}

	parking := &domain.Parking{
		Name:       name,
		Address:    address,
		TotalSpots: totalSpots,
	}

	err := u.parkingRepo.Create(parking)
	if err != nil {
		return nil, err
	}

	return parking, nil
}

func (u *ParkingUsecase) GetParking(id int64) (*domain.Parking, error) {
	return u.parkingRepo.GetByID(id)
}

func (u *ParkingUsecase) GetAllParkings() ([]domain.Parking, error) {
	return u.parkingRepo.GetAll()
}

func (u *ParkingUsecase) DeleteParking(id int64) error {
	return u.parkingRepo.Delete(id)
}
