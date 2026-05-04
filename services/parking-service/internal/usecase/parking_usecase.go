package usecase

import (
	"parking-service/internal/domain"
	"parking-service/internal/repository"
)

type ParkingUsecase struct {
	parkingRepo *repository.ParkingRepository
}

func NewParkingUsecase(parkingRepo *repository.ParkingRepository) *ParkingUsecase {
	return &ParkingUsecase{parkingRepo: parkingRepo}
}

func (u *ParkingUsecase) CreateParking(name, address string, totalSpots int) (*domain.Parking, error) {
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
