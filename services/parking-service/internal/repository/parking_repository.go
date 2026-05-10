package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	cacheLotKey  = "parking:lot:"
	cacheLotsKey = "parking:lots"
)

type ParkingRepository struct {
	db    *sql.DB
	cache *redis.Client
	ttl   time.Duration
}

func NewParkingRepository(db *sql.DB, cache *redis.Client, ttl time.Duration) *ParkingRepository {
	return &ParkingRepository{db: db, cache: cache, ttl: ttl}
}

func (r *ParkingRepository) Create(parking *domain.Parking) error {
	query := `
		INSERT INTO parkings (name, address, total_spots)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	if err := r.db.QueryRow(query, parking.Name, parking.Address, parking.TotalSpots).Scan(&parking.ID, &parking.CreatedAt); err != nil {
		return err
	}

	r.invalidateListCache(context.Background())
	r.setCache(context.Background(), parking)
	return nil
}

func (r *ParkingRepository) GetByID(id int64) (*domain.Parking, error) {
	ctx := context.Background()
	if parking, ok := r.getCache(ctx, cacheLotKey+strconv.FormatInt(id, 10)); ok {
		return parking, nil
	}

	query := `
		SELECT id, name, address, total_spots, created_at
		FROM parkings
		WHERE id = $1
	`

	var parking domain.Parking
	if err := r.db.QueryRow(query, id).Scan(&parking.ID, &parking.Name, &parking.Address, &parking.TotalSpots, &parking.CreatedAt); err != nil {
		return nil, err
	}

	r.setCache(ctx, &parking)
	return &parking, nil
}

func (r *ParkingRepository) GetAll() ([]domain.Parking, error) {
	ctx := context.Background()
	if cached, ok := r.getAllCache(ctx); ok {
		return cached, nil
	}

	rows, err := r.db.Query(`
		SELECT id, name, address, total_spots, created_at
		FROM parkings
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parkings := make([]domain.Parking, 0)
	for rows.Next() {
		var parking domain.Parking
		if err := rows.Scan(&parking.ID, &parking.Name, &parking.Address, &parking.TotalSpots, &parking.CreatedAt); err != nil {
			return nil, err
		}
		parkings = append(parkings, parking)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.setAllCache(ctx, parkings)
	return parkings, nil
}

func (r *ParkingRepository) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM parkings WHERE id = $1`, id); err != nil {
		return err
	}

	ctx := context.Background()
	r.invalidateListCache(ctx)
	r.invalidateCache(ctx, cacheLotKey+strconv.FormatInt(id, 10))
	return nil
}

func (r *ParkingRepository) getCache(ctx context.Context, key string) (*domain.Parking, bool) {
	if r.cache == nil {
		return nil, false
	}

	data, err := r.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var parking domain.Parking
	if err := json.Unmarshal(data, &parking); err != nil {
		return nil, false
	}

	return &parking, true
}

func (r *ParkingRepository) setCache(ctx context.Context, parking *domain.Parking) {
	if r.cache == nil || parking == nil {
		return
	}

	data, err := json.Marshal(parking)
	if err != nil {
		log.Printf("parking cache marshal error: %v", err)
		return
	}

	if err := r.cache.Set(ctx, cacheLotKey+strconv.FormatInt(parking.ID, 10), data, r.ttl).Err(); err != nil {
		log.Printf("parking cache set error: %v", err)
	}
}

func (r *ParkingRepository) getAllCache(ctx context.Context) ([]domain.Parking, bool) {
	if r.cache == nil {
		return nil, false
	}

	data, err := r.cache.Get(ctx, cacheLotsKey).Bytes()
	if err != nil {
		return nil, false
	}

	var parkings []domain.Parking
	if err := json.Unmarshal(data, &parkings); err != nil {
		return nil, false
	}

	return parkings, true
}

func (r *ParkingRepository) setAllCache(ctx context.Context, parkings []domain.Parking) {
	if r.cache == nil {
		return
	}

	data, err := json.Marshal(parkings)
	if err != nil {
		log.Printf("parking list cache marshal error: %v", err)
		return
	}

	if err := r.cache.Set(ctx, cacheLotsKey, data, r.ttl).Err(); err != nil {
		log.Printf("parking list cache set error: %v", err)
	}
}

func (r *ParkingRepository) invalidateCache(ctx context.Context, key string) {
	if r.cache == nil {
		return
	}
	_, _ = r.cache.Del(ctx, key).Result()
}

func (r *ParkingRepository) invalidateListCache(ctx context.Context) {
	if r.cache == nil {
		return
	}
	_, _ = r.cache.Del(ctx, cacheLotsKey).Result()
}
