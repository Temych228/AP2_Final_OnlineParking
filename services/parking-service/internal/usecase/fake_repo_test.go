package usecase_test

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
)

type fakeParkingDB struct {
	mu      sync.Mutex
	records map[int64]*domain.Parking
	nextID  int64
}

func newFakeParkingDB() *fakeParkingDB {
	return &fakeParkingDB{records: make(map[int64]*domain.Parking), nextID: 1}
}

func (f *fakeParkingDB) Create(p *domain.Parking) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p.ID = f.nextID
	f.nextID++
	cp := *p
	f.records[p.ID] = &cp
	return nil
}

func (f *fakeParkingDB) GetByID(id int64) (*domain.Parking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.records[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeParkingDB) GetAll() ([]domain.Parking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Parking, 0, len(f.records))
	for _, p := range f.records {
		result = append(result, *p)
	}
	return result, nil
}

func (f *fakeParkingDB) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.records[id]; !ok {
		return sql.ErrNoRows
	}
	delete(f.records, id)
	return nil
}

type fakeSpotDB struct {
	mu     sync.Mutex
	spots  map[int64]*domain.Spot
	nextID int64
}

func newFakeSpotDB() *fakeSpotDB {
	return &fakeSpotDB{spots: make(map[int64]*domain.Spot), nextID: 1}
}

func (f *fakeSpotDB) Create(s *domain.Spot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s.ID = f.nextID
	f.nextID++
	cp := *s
	f.spots[s.ID] = &cp
	return nil
}

func (f *fakeSpotDB) GetByID(id int64) (*domain.Spot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.spots[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeSpotDB) GetByParkingID(parkingID int64) ([]domain.Spot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Spot
	for _, s := range f.spots {
		if s.ParkingID == parkingID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (f *fakeSpotDB) UpdateStatus(id int64, status domain.SpotStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.spots[id]
	if !ok {
		return sql.ErrNoRows
	}
	s.Status = status
	return nil
}

func (f *fakeSpotDB) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.spots[id]; !ok {
		return sql.ErrNoRows
	}
	delete(f.spots, id)
	return nil
}

func (f *fakeSpotDB) CountByParkingID(parkingID int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, s := range f.spots {
		if s.ParkingID == parkingID {
			count++
		}
	}
	return count, nil
}

type fakeTariffDB struct {
	mu      sync.Mutex
	tariffs map[int64]*domain.Tariff
	nextID  int64
}

func newFakeTariffDB() *fakeTariffDB {
	return &fakeTariffDB{tariffs: make(map[int64]*domain.Tariff), nextID: 1}
}

func (f *fakeTariffDB) Create(t *domain.Tariff) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t.ID = f.nextID
	f.nextID++
	cp := *t
	f.tariffs[t.ID] = &cp
	return nil
}

func (f *fakeTariffDB) GetByParkingID(parkingID int64) (*domain.Tariff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tariffs {
		if t.ParkingID == parkingID {
			cp := *t
			return &cp, nil
		}
	}
	return nil, errors.New("tariff not found")
}

func (f *fakeTariffDB) GetAll() ([]domain.Tariff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Tariff, 0, len(f.tariffs))
	for _, t := range f.tariffs {
		result = append(result, *t)
	}
	return result, nil
}
