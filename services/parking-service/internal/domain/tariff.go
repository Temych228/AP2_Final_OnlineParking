package domain

import "time"

type Tariff struct {
	ID           int64
	ParkingID    int64
	PricePerHour float64
	CreatedAt    time.Time
}
