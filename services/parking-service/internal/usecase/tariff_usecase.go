package usecase

import (
	"parking-service/internal/domain"
	"parking-service/internal/repository"
)

type TariffUsecase struct {
	tariffRepo *repository.TariffRepository
}

func NewTariffUsecase(tariffRepo *repository.TariffRepository) *TariffUsecase {
	return &TariffUsecase{tariffRepo: tariffRepo}
}

func (u *TariffUsecase) CreateTariff(parkingID int64, pricePerHour float64) (*domain.Tariff, error) {
	tariff := &domain.Tariff{
		ParkingID:    parkingID,
		PricePerHour: pricePerHour,
	}

	err := u.tariffRepo.Create(tariff)
	if err != nil {
		return nil, err
	}

	return tariff, nil
}

func (u *TariffUsecase) GetTariff(parkingID int64) (*domain.Tariff, error) {
	return u.tariffRepo.GetByParkingID(parkingID)
}

func (u *TariffUsecase) UpdateTariff(parkingID int64, pricePerHour float64) error {
	return u.tariffRepo.UpdatePrice(parkingID, pricePerHour)
}

func (u *TariffUsecase) CalculatePrice(parkingID int64, hours float64) (float64, error) {
	tariff, err := u.tariffRepo.GetByParkingID(parkingID)
	if err != nil {
		return 0, err
	}

	return tariff.PricePerHour * hours, nil
}
