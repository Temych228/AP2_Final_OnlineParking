package usecase

import (
	"errors"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
)

type SpotRepo interface {
	Create(spot *domain.Spot) error
	GetByID(id int64) (*domain.Spot, error)
	GetByParkingID(parkingID int64) ([]domain.Spot, error)
	UpdateStatus(id int64, status domain.SpotStatus) error
	Delete(id int64) error
	CountByParkingID(parkingID int64) (int, error)
}

type SpotUsecase struct {
	spotRepo    SpotRepo
	parkingRepo ParkingRepo
}

func NewSpotUsecase(spotRepo SpotRepo, parkingRepo ParkingRepo) *SpotUsecase {
	return &SpotUsecase{spotRepo: spotRepo, parkingRepo: parkingRepo}
}

func (u *SpotUsecase) CreateSpot(parkingID int64, number string) (*domain.Spot, error) {
	number = strings.TrimSpace(number)

	if parkingID <= 0 {
		return nil, errors.New("parking_id is required")
	}
	if number == "" {
		return nil, errors.New("spot number is required")
	}

	parking, err := u.parkingRepo.GetByID(parkingID)
	if err != nil {
		return nil, errors.New("parking not found")
	}

	currentCount, err := u.spotRepo.CountByParkingID(parkingID)
	if err != nil {
		return nil, err
	}
	if currentCount >= parking.TotalSpots {
		return nil, errors.New("parking spot limit reached")
	}

	spot := &domain.Spot{
		ParkingID: parkingID,
		Number:    number,
		Status:    domain.StatusAvailable,
	}
	if err := u.spotRepo.Create(spot); err != nil {
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
