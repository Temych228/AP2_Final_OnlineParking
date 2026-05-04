package repository

import (
	"database/sql"

	"parking-service/internal/domain"
)

type ParkingRepository struct {
	db *sql.DB
}

func NewParkingRepository(db *sql.DB) *ParkingRepository {
	return &ParkingRepository{db: db}
}

func (r *ParkingRepository) Create(parking *domain.Parking) error {
	query := `
		INSERT INTO parkings (name, address, total_spots)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	return r.db.QueryRow(
		query,
		parking.Name,
		parking.Address,
		parking.TotalSpots,
	).Scan(&parking.ID, &parking.CreatedAt)
}

func (r *ParkingRepository) GetByID(id int64) (*domain.Parking, error) {
	query := `
		SELECT id, name, address, total_spots, created_at
		FROM parkings
		WHERE id = $1
	`

	var parking domain.Parking

	err := r.db.QueryRow(query, id).Scan(
		&parking.ID,
		&parking.Name,
		&parking.Address,
		&parking.TotalSpots,
		&parking.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &parking, nil
}

func (r *ParkingRepository) GetAll() ([]domain.Parking, error) {
	query := `
		SELECT id, name, address, total_spots, created_at
		FROM parkings
		ORDER BY id
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parkings []domain.Parking

	for rows.Next() {
		var parking domain.Parking

		err := rows.Scan(
			&parking.ID,
			&parking.Name,
			&parking.Address,
			&parking.TotalSpots,
			&parking.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		parkings = append(parkings, parking)
	}

	return parkings, nil
}

func (r *ParkingRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM parkings WHERE id = $1`, id)
	return err
}
