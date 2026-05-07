package repository

import (
	"database/sql"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
)

type TariffRepository struct {
	db *sql.DB
}

func NewTariffRepository(db *sql.DB) *TariffRepository {
	return &TariffRepository{db: db}
}

func (r *TariffRepository) Create(tariff *domain.Tariff) error {
	query := `
		INSERT INTO tariffs (parking_id, price_per_hour)
		VALUES ($1, $2)
		RETURNING id, created_at
	`

	return r.db.QueryRow(
		query,
		tariff.ParkingID,
		tariff.PricePerHour,
	).Scan(&tariff.ID, &tariff.CreatedAt)
}

func (r *TariffRepository) GetByParkingID(parkingID int64) (*domain.Tariff, error) {
	query := `
		SELECT id, parking_id, price_per_hour, created_at
		FROM tariffs
		WHERE parking_id = $1
	`

	var tariff domain.Tariff

	err := r.db.QueryRow(query, parkingID).Scan(
		&tariff.ID,
		&tariff.ParkingID,
		&tariff.PricePerHour,
		&tariff.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &tariff, nil
}

func (r *TariffRepository) UpdatePrice(parkingID int64, pricePerHour float64) error {
	_, err := r.db.Exec(
		`UPDATE tariffs SET price_per_hour = $1 WHERE parking_id = $2`,
		pricePerHour,
		parkingID,
	)

	return err
}
