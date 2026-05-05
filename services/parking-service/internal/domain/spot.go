package domain

import "time"

type SpotStatus string

const (
	StatusAvailable   SpotStatus = "AVAILABLE"
	StatusReserved    SpotStatus = "RESERVED"
	StatusOccupied    SpotStatus = "OCCUPIED"
	StatusMaintenance SpotStatus = "MAINTENANCE"
)

type Spot struct {
	ID        int64
	ParkingID int64
	Number    string
	Status    SpotStatus
	CreatedAt time.Time
}
