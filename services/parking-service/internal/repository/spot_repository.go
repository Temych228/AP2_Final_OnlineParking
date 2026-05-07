package repository

import (
	"database/sql"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
)

type SpotRepository struct {
	db *sql.DB
}

func NewSpotRepository(db *sql.DB) *SpotRepository {
	return &SpotRepository{db: db}
}

func (r *SpotRepository) Create(spot *domain.Spot) error {
	query := `
		INSERT INTO spots (parking_id, number, status)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	return r.db.QueryRow(
		query,
		spot.ParkingID,
		spot.Number,
		spot.Status,
	).Scan(&spot.ID, &spot.CreatedAt)
}

func (r *SpotRepository) GetByID(id int64) (*domain.Spot, error) {
	query := `
		SELECT id, parking_id, number, status, created_at
		FROM spots
		WHERE id = $1
	`

	var spot domain.Spot

	err := r.db.QueryRow(query, id).Scan(
		&spot.ID,
		&spot.ParkingID,
		&spot.Number,
		&spot.Status,
		&spot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &spot, nil
}

func (r *SpotRepository) GetByParkingID(parkingID int64) ([]domain.Spot, error) {
	query := `
		SELECT id, parking_id, number, status, created_at
		FROM spots
		WHERE parking_id = $1
		ORDER BY id
	`

	rows, err := r.db.Query(query, parkingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spots []domain.Spot

	for rows.Next() {
		var spot domain.Spot

		err := rows.Scan(
			&spot.ID,
			&spot.ParkingID,
			&spot.Number,
			&spot.Status,
			&spot.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		spots = append(spots, spot)
	}

	return spots, nil
}

func (r *SpotRepository) UpdateStatus(id int64, status domain.SpotStatus) error {
	_, err := r.db.Exec(
		`UPDATE spots SET status = $1 WHERE id = $2`,
		status,
		id,
	)

	return err
}

func (r *SpotRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM spots WHERE id = $1`, id)
	return err
}
