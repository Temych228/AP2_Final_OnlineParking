package usecase

import (
	"errors"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
)

type TariffUsecase struct {
	tariffRepo  *repository.TariffRepository
	parkingRepo *repository.ParkingRepository
}

func NewTariffUsecase(
	tariffRepo *repository.TariffRepository,
	parkingRepo *repository.ParkingRepository,
) *TariffUsecase {
	return &TariffUsecase{
		tariffRepo:  tariffRepo,
		parkingRepo: parkingRepo,
	}
}

func (u *TariffUsecase) CreateTariff(parkingID int64, pricePerHour float64) (*domain.Tariff, error) {
	if parkingID <= 0 {
		return nil, errors.New("parking_id is required")
	}

	if pricePerHour <= 0 {
		return nil, errors.New("price_per_hour must be greater than zero")
	}

	_, err := u.parkingRepo.GetByID(parkingID)
	if err != nil {
		return nil, errors.New("parking not found")
	}

	tariff := &domain.Tariff{
		ParkingID:    parkingID,
		PricePerHour: pricePerHour,
	}

	err = u.tariffRepo.Create(tariff)
	if err != nil {
		return nil, err
	}

	return tariff, nil
}

func (u *TariffUsecase) GetTariff(parkingID int64) (*domain.Tariff, error) {
	if parkingID <= 0 {
		return nil, errors.New("parking_id is required")
	}

	return u.tariffRepo.GetByParkingID(parkingID)
}

func (u *TariffUsecase) UpdateTariff(parkingID int64, pricePerHour float64) error {
	if parkingID <= 0 {
		return errors.New("parking_id is required")
	}

	if pricePerHour <= 0 {
		return errors.New("price_per_hour must be greater than zero")
	}

	_, err := u.parkingRepo.GetByID(parkingID)
	if err != nil {
		return errors.New("parking not found")
	}

	return u.tariffRepo.UpdatePrice(parkingID, pricePerHour)
}

func (u *TariffUsecase) CalculatePrice(parkingID int64, hours float64) (float64, error) {
	if parkingID <= 0 {
		return 0, errors.New("parking_id is required")
	}

	if hours <= 0 {
		return 0, errors.New("hours must be greater than zero")
	}

	tariff, err := u.tariffRepo.GetByParkingID(parkingID)
	if err != nil {
		return 0, err
	}

	return tariff.PricePerHour * hours, nil
}
