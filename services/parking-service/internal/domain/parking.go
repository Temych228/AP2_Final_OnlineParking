package domain

import "time"

type Parking struct {
	ID         int64
	Name       string
	Address    string
	TotalSpots int
	CreatedAt  time.Time
}
