package grpcserver

import (
	"context"
	"errors"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
	"strconv"
	"strings"

	parkingv1 "github.com/Temych228/ap2_protos_go_final/parking/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service interface {
	CreateParkingLot(ctx context.Context, in CreateParkingLotInput) (*ParkingLotOutput, error)
	GetParkingLot(ctx context.Context, parkingLotID string) (*ParkingLotOutput, error)
	UpdateParkingLot(ctx context.Context, in UpdateParkingLotInput) (*ParkingLotOutput, error)
	DeleteParkingLot(ctx context.Context, parkingLotID string) error
	ListParkingLots(ctx context.Context, in ListParkingLotsInput) ([]ParkingLotOutput, int, error)

	CreateParkingSpot(ctx context.Context, in CreateParkingSpotInput) (*ParkingSpotOutput, error)
	GetParkingSpot(ctx context.Context, parkingSpotID string) (*ParkingSpotOutput, error)
	UpdateParkingSpot(ctx context.Context, in UpdateParkingSpotInput) (*ParkingSpotOutput, error)
	DeleteParkingSpot(ctx context.Context, parkingSpotID string) error
	ListParkingSpots(ctx context.Context, in ListParkingSpotsInput) ([]ParkingSpotOutput, int, error)

	ReserveSpot(ctx context.Context, in ReserveSpotInput) (*ReservationOutput, *ParkingSpotOutput, error)
	ReleaseSpot(ctx context.Context, reservationID, userID string) (*ReservationOutput, error)
}

type CreateParkingLotInput struct {
	Name         string
	Address      string
	Latitude     float64
	Longitude    float64
	TotalSpots   int32
	PricePerHour float64
}

type UpdateParkingLotInput struct {
	ParkingLotID string
	Name         string
	Address      string
	Latitude     float64
	Longitude    float64
	TotalSpots   int32
	PricePerHour float64
	IsActive     bool
}

type ListParkingLotsInput struct {
	Page       int32
	PageSize   int32
	ActiveOnly bool
}

type CreateParkingSpotInput struct {
	ParkingLotID string
	Code         string
	Level        string
	SpotType     string
	VehicleType  string
}

type UpdateParkingSpotInput struct {
	ParkingSpotID string
	Code          string
	Level         string
	SpotType      string
	VehicleType   string
	IsActive      bool
}

type ListParkingSpotsInput struct {
	ParkingLotID  string
	Page          int32
	PageSize      int32
	OnlyAvailable bool
	VehicleType   string
}

type ReserveSpotInput struct {
	UserID        string
	ParkingLotID  string
	ParkingSpotID string
	VehicleNumber string
	StartsAt      *timestamppb.Timestamp
	EndsAt        *timestamppb.Timestamp
}

type ParkingLotOutput struct {
	ID             string
	Name           string
	Address        string
	Latitude       float64
	Longitude      float64
	TotalSpots     int32
	AvailableSpots int32
	PricePerHour   float64
	IsActive       bool
	CreatedAt      string
	UpdatedAt      string
}

type ParkingSpotOutput struct {
	ID           string
	ParkingLotID string
	Code         string
	Level        string
	SpotType     string
	VehicleType  string
	IsActive     bool
	IsOccupied   bool
	IsReserved   bool
	CreatedAt    string
	UpdatedAt    string
}

type ReservationOutput struct {
	ID            string
	UserID        string
	ParkingLotID  string
	ParkingSpotID string
	VehicleNumber string
	Status        string
	StartsAt      string
	EndsAt        string
	CreatedAt     string
	UpdatedAt     string
}

type Server struct {
	parkingv1.UnimplementedParkingServiceServer
	svc Service
}

type ParkingGRPCHandler struct {
	parkingv1.UnimplementedParkingServiceServer
	parkingUC *usecase.ParkingUsecase
	spotUC    *usecase.SpotUsecase
	tariffUC  *usecase.TariffUsecase
}

func New(svc Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) CreateParkingLot(ctx context.Context, req *parkingv1.CreateParkingLotRequest) (*parkingv1.CreateParkingLotResponse, error) {
	if err := validateCreateParkingLot(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	out, err := s.svc.CreateParkingLot(ctx, CreateParkingLotInput{
		Name:         req.GetName(),
		Address:      req.GetAddress(),
		Latitude:     req.GetLatitude(),
		Longitude:    req.GetLongitude(),
		TotalSpots:   req.GetTotalSpots(),
		PricePerHour: req.GetPricePerHour(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.CreateParkingLotResponse{
		ParkingLot: toProtoParkingLot(out),
	}, nil
}

func (s *Server) GetParkingLot(ctx context.Context, req *parkingv1.GetParkingLotRequest) (*parkingv1.GetParkingLotResponse, error) {
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}

	out, err := s.svc.GetParkingLot(ctx, req.GetParkingLotId())
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.GetParkingLotResponse{
		ParkingLot: toProtoParkingLot(out),
	}, nil
}

func (s *Server) UpdateParkingLot(ctx context.Context, req *parkingv1.UpdateParkingLotRequest) (*parkingv1.UpdateParkingLotResponse, error) {
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}

	out, err := s.svc.UpdateParkingLot(ctx, UpdateParkingLotInput{
		ParkingLotID: req.GetParkingLotId(),
		Name:         req.GetName(),
		Address:      req.GetAddress(),
		Latitude:     req.GetLatitude(),
		Longitude:    req.GetLongitude(),
		TotalSpots:   req.GetTotalSpots(),
		PricePerHour: req.GetPricePerHour(),
		IsActive:     req.GetIsActive(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.UpdateParkingLotResponse{
		ParkingLot: toProtoParkingLot(out),
	}, nil
}

func (s *Server) DeleteParkingLot(ctx context.Context, req *parkingv1.DeleteParkingLotRequest) (*parkingv1.DeleteParkingLotResponse, error) {
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}

	if err := s.svc.DeleteParkingLot(ctx, req.GetParkingLotId()); err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.DeleteParkingLotResponse{Success: true}, nil
}

func (s *Server) ListParkingLots(ctx context.Context, req *parkingv1.ListParkingLotsRequest) (*parkingv1.ListParkingLotsResponse, error) {
	items, total, err := s.svc.ListParkingLots(ctx, ListParkingLotsInput{
		Page:       req.GetPage(),
		PageSize:   req.GetPageSize(),
		ActiveOnly: req.GetActiveOnly(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*parkingv1.ParkingLot, 0, len(items))
	for i := range items {
		item := items[i]
		out = append(out, toProtoParkingLot(&item))
	}

	return &parkingv1.ListParkingLotsResponse{
		ParkingLots: out,
		Total:       int32(total),
	}, nil
}

func (s *Server) CreateParkingSpot(ctx context.Context, req *parkingv1.CreateParkingSpotRequest) (*parkingv1.CreateParkingSpotResponse, error) {
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}
	if strings.TrimSpace(req.GetCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	out, err := s.svc.CreateParkingSpot(ctx, CreateParkingSpotInput{
		ParkingLotID: req.GetParkingLotId(),
		Code:         req.GetCode(),
		Level:        req.GetLevel(),
		SpotType:     req.GetSpotType(),
		VehicleType:  req.GetVehicleType(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.CreateParkingSpotResponse{
		ParkingSpot: toProtoParkingSpot(out),
	}, nil
}

func (s *Server) GetParkingSpot(ctx context.Context, req *parkingv1.GetParkingSpotRequest) (*parkingv1.GetParkingSpotResponse, error) {
	if strings.TrimSpace(req.GetParkingSpotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_spot_id is required")
	}

	out, err := s.svc.GetParkingSpot(ctx, req.GetParkingSpotId())
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.GetParkingSpotResponse{
		ParkingSpot: toProtoParkingSpot(out),
	}, nil
}

func (s *Server) UpdateParkingSpot(ctx context.Context, req *parkingv1.UpdateParkingSpotRequest) (*parkingv1.UpdateParkingSpotResponse, error) {
	if strings.TrimSpace(req.GetParkingSpotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_spot_id is required")
	}

	out, err := s.svc.UpdateParkingSpot(ctx, UpdateParkingSpotInput{
		ParkingSpotID: req.GetParkingSpotId(),
		Code:          req.GetCode(),
		Level:         req.GetLevel(),
		SpotType:      req.GetSpotType(),
		VehicleType:   req.GetVehicleType(),
		IsActive:      req.GetIsActive(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.UpdateParkingSpotResponse{
		ParkingSpot: toProtoParkingSpot(out),
	}, nil
}

func (s *Server) DeleteParkingSpot(ctx context.Context, req *parkingv1.DeleteParkingSpotRequest) (*parkingv1.DeleteParkingSpotResponse, error) {
	if strings.TrimSpace(req.GetParkingSpotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_spot_id is required")
	}

	if err := s.svc.DeleteParkingSpot(ctx, req.GetParkingSpotId()); err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.DeleteParkingSpotResponse{Success: true}, nil
}

func (s *Server) ListParkingSpots(ctx context.Context, req *parkingv1.ListParkingSpotsRequest) (*parkingv1.ListParkingSpotsResponse, error) {
	items, total, err := s.svc.ListParkingSpots(ctx, ListParkingSpotsInput{
		ParkingLotID:  req.GetParkingLotId(),
		Page:          req.GetPage(),
		PageSize:      req.GetPageSize(),
		OnlyAvailable: req.GetOnlyAvailable(),
		VehicleType:   req.GetVehicleType(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*parkingv1.ParkingSpot, 0, len(items))
	for i := range items {
		item := items[i]
		out = append(out, toProtoParkingSpot(&item))
	}

	return &parkingv1.ListParkingSpotsResponse{
		ParkingSpots: out,
		Total:        int32(total),
	}, nil
}

func (s *Server) ReserveSpot(ctx context.Context, req *parkingv1.ReserveSpotRequest) (*parkingv1.ReserveSpotResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}
	if strings.TrimSpace(req.GetParkingSpotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_spot_id is required")
	}
	if req.GetStartsAt() == nil || req.GetEndsAt() == nil {
		return nil, status.Error(codes.InvalidArgument, "starts_at and ends_at are required")
	}

	reservation, spot, err := s.svc.ReserveSpot(ctx, ReserveSpotInput{
		UserID:        req.GetUserId(),
		ParkingLotID:  req.GetParkingLotId(),
		ParkingSpotID: req.GetParkingSpotId(),
		VehicleNumber: req.GetVehicleNumber(),
		StartsAt:      req.GetStartsAt(),
		EndsAt:        req.GetEndsAt(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.ReserveSpotResponse{
		Reservation: toProtoReservation(reservation),
		ParkingSpot: toProtoParkingSpot(spot),
	}, nil
}

func (s *Server) ReleaseSpot(ctx context.Context, req *parkingv1.ReleaseSpotRequest) (*parkingv1.ReleaseSpotResponse, error) {
	if strings.TrimSpace(req.GetReservationId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "reservation_id is required")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	reservation, err := s.svc.ReleaseSpot(ctx, req.GetReservationId(), req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &parkingv1.ReleaseSpotResponse{
		Success:     true,
		Reservation: toProtoReservation(reservation),
	}, nil
}

func validateCreateParkingLot(req *parkingv1.CreateParkingLotRequest) error {
	switch {
	case strings.TrimSpace(req.GetName()) == "":
		return errors.New("name is required")
	case strings.TrimSpace(req.GetAddress()) == "":
		return errors.New("address is required")
	case req.GetTotalSpots() <= 0:
		return errors.New("total_spots must be greater than zero")
	case req.GetPricePerHour() < 0:
		return errors.New("price_per_hour cannot be negative")
	default:
		return nil
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(strings.ToLower(err.Error()), "not found"):
		return status.Error(codes.NotFound, err.Error())
	case strings.Contains(strings.ToLower(err.Error()), "invalid"):
		return status.Error(codes.InvalidArgument, err.Error())
	case strings.Contains(strings.ToLower(err.Error()), "conflict"):
		return status.Error(codes.AlreadyExists, err.Error())
	case strings.Contains(strings.ToLower(err.Error()), "forbidden"):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func toProtoParkingLot(out *ParkingLotOutput) *parkingv1.ParkingLot {
	if out == nil {
		return nil
	}

	return &parkingv1.ParkingLot{
		Id:             out.ID,
		Name:           out.Name,
		Address:        out.Address,
		Latitude:       out.Latitude,
		Longitude:      out.Longitude,
		TotalSpots:     out.TotalSpots,
		AvailableSpots: out.AvailableSpots,
		PricePerHour:   out.PricePerHour,
		IsActive:       out.IsActive,
	}
}

func toProtoParkingSpot(out *ParkingSpotOutput) *parkingv1.ParkingSpot {
	if out == nil {
		return nil
	}

	return &parkingv1.ParkingSpot{
		Id:           out.ID,
		ParkingLotId: out.ParkingLotID,
		Code:         out.Code,
		Level:        out.Level,
		SpotType:     out.SpotType,
		VehicleType:  out.VehicleType,
		IsActive:     out.IsActive,
		IsOccupied:   out.IsOccupied,
		IsReserved:   out.IsReserved,
	}
}

func toProtoReservation(out *ReservationOutput) *parkingv1.Reservation {
	if out == nil {
		return nil
	}

	return &parkingv1.Reservation{
		Id:            out.ID,
		UserId:        out.UserID,
		ParkingLotId:  out.ParkingLotID,
		ParkingSpotId: out.ParkingSpotID,
		VehicleNumber: out.VehicleNumber,
		Status:        out.Status,
	}
}

func NewParkingGRPCHandler(
	parkingUC *usecase.ParkingUsecase,
	spotUC *usecase.SpotUsecase,
	tariffUC *usecase.TariffUsecase,
) parkingv1.ParkingServiceServer {
	return &ParkingGRPCHandler{
		parkingUC: parkingUC,
		spotUC:    spotUC,
		tariffUC:  tariffUC,
	}
}

func (h *ParkingGRPCHandler) GetParkingLot(ctx context.Context, req *parkingv1.GetParkingLotRequest) (*parkingv1.GetParkingLotResponse, error) {
	if strings.TrimSpace(req.GetParkingLotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_lot_id is required")
	}

	id, err := strconv.ParseInt(req.GetParkingLotId(), 10, 64)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid parking_lot_id")
	}

	parking, err := h.parkingUC.GetParking(id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "parking lot not found")
	}

	return &parkingv1.GetParkingLotResponse{
		ParkingLot: &parkingv1.ParkingLot{
			Id:             strconv.FormatInt(parking.ID, 10),
			Name:           parking.Name,
			Address:        parking.Address,
			TotalSpots:     int32(parking.TotalSpots),
			AvailableSpots: int32(parking.TotalSpots),
			PricePerHour:   0,
			IsActive:       true,
		},
	}, nil
}

func (h *ParkingGRPCHandler) GetParkingSpot(ctx context.Context, req *parkingv1.GetParkingSpotRequest) (*parkingv1.GetParkingSpotResponse, error) {
	if strings.TrimSpace(req.GetParkingSpotId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_spot_id is required")
	}

	id, err := strconv.ParseInt(req.GetParkingSpotId(), 10, 64)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid parking_spot_id")
	}

	spot, err := h.spotUC.GetSpot(id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "parking spot not found")
	}

	return &parkingv1.GetParkingSpotResponse{
		ParkingSpot: &parkingv1.ParkingSpot{
			Id:           strconv.FormatInt(spot.ID, 10),
			ParkingLotId: strconv.FormatInt(spot.ParkingID, 10),
			Code:         spot.Number,
			Level:        "1",
			SpotType:     "standard",
			VehicleType:  "car",
			IsActive:     true,
			IsOccupied:   string(spot.Status) == "OCCUPIED",
			IsReserved:   string(spot.Status) == "RESERVED",
		},
	}, nil
}
