package usecase

import (
	"errors"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
)

type SpotUsecase struct {
	spotRepo *repository.SpotRepository
}

func NewSpotUsecase(spotRepo *repository.SpotRepository) *SpotUsecase {
	return &SpotUsecase{spotRepo: spotRepo}
}

func (u *SpotUsecase) CreateSpot(parkingID int64, number string) (*domain.Spot, error) {
	spot := &domain.Spot{
		ParkingID: parkingID,
		Number:    number,
		Status:    domain.StatusAvailable,
	}

	err := u.spotRepo.Create(spot)
	if err != nil {
		return nil, err
	}

	return spot, nil
}

func (u *SpotUsecase) GetSpot(id int64) (*domain.Spot, error) {
	return u.spotRepo.GetByID(id)
}

func (u *SpotUsecase) GetSpotsByParking(parkingID int64) ([]domain.Spot, error) {
	return u.spotRepo.GetByParkingID(parkingID)
}

func (u *SpotUsecase) CheckAvailability(spotID int64) (bool, error) {
	spot, err := u.spotRepo.GetByID(spotID)
	if err != nil {
		return false, err
	}

	return spot.Status == domain.StatusAvailable, nil
}

func (u *SpotUsecase) ReserveSpot(spotID int64) error {
	spot, err := u.spotRepo.GetByID(spotID)
	if err != nil {
		return err
	}

	if spot.Status != domain.StatusAvailable {
		return errors.New("spot is not available")
	}

	return u.spotRepo.UpdateStatus(spotID, domain.StatusReserved)
}

func (u *SpotUsecase) ReleaseSpot(spotID int64) error {
	spot, err := u.spotRepo.GetByID(spotID)
	if err != nil {
		return err
	}

	if spot.Status != domain.StatusReserved {
		return errors.New("spot is not reserved")
	}

	return u.spotRepo.UpdateStatus(spotID, domain.StatusAvailable)
}

func (u *SpotUsecase) UpdateSpotStatus(spotID int64, status domain.SpotStatus) error {
	return u.spotRepo.UpdateStatus(spotID, status)
}

func (u *SpotUsecase) DeleteSpot(spotID int64) error {
	return u.spotRepo.Delete(spotID)
}
