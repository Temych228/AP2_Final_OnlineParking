package grpc

import (
	"context"

	"parking-service/internal/domain"
	"parking-service/internal/usecase"

	parkingv1 "github.com/Temych228/ap2_protos_go_final/parking/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ParkingGRPCHandler struct {
	parkingv1.UnimplementedParkingServiceServer

	parkingUC *usecase.ParkingUsecase
	spotUC    *usecase.SpotUsecase
	tariffUC  *usecase.TariffUsecase
}

func NewParkingGRPCHandler(
	parkingUC *usecase.ParkingUsecase,
	spotUC *usecase.SpotUsecase,
	tariffUC *usecase.TariffUsecase,
) *ParkingGRPCHandler {
	return &ParkingGRPCHandler{
		parkingUC: parkingUC,
		spotUC:    spotUC,
		tariffUC:  tariffUC,
	}
}

func (h *ParkingGRPCHandler) CreateParking(ctx context.Context, req *parkingv1.CreateParkingRequest) (*parkingv1.ParkingResponse, error) {
	if req.GetName() == "" || req.GetAddress() == "" || req.GetTotalSpots() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "name, address and total_spots are required")
	}

	parking, err := h.parkingUC.CreateParking(req.GetName(), req.GetAddress(), int(req.GetTotalSpots()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &parkingv1.ParkingResponse{Parking: toProtoParking(parking)}, nil
}

func (h *ParkingGRPCHandler) GetParking(ctx context.Context, req *parkingv1.GetParkingRequest) (*parkingv1.ParkingResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking id is required")
	}

	parking, err := h.parkingUC.GetParking(req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "parking not found")
	}

	return &parkingv1.ParkingResponse{Parking: toProtoParking(parking)}, nil
}

func (h *ParkingGRPCHandler) GetAllParkings(ctx context.Context, req *parkingv1.GetAllParkingsRequest) (*parkingv1.ParkingListResponse, error) {
	parkings, err := h.parkingUC.GetAllParkings()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	result := make([]*parkingv1.Parking, 0, len(parkings))
	for _, parking := range parkings {
		p := parking
		result = append(result, toProtoParking(&p))
	}

	return &parkingv1.ParkingListResponse{Parkings: result}, nil
}

func (h *ParkingGRPCHandler) DeleteParking(ctx context.Context, req *parkingv1.DeleteParkingRequest) (*parkingv1.ActionResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking id is required")
	}

	if err := h.parkingUC.DeleteParking(req.GetId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return actionOK("parking deleted"), nil
}

func (h *ParkingGRPCHandler) CreateSpot(ctx context.Context, req *parkingv1.CreateSpotRequest) (*parkingv1.SpotResponse, error) {
	if req.GetParkingId() <= 0 || req.GetNumber() == "" {
		return nil, status.Error(codes.InvalidArgument, "parking_id and number are required")
	}

	spot, err := h.spotUC.CreateSpot(req.GetParkingId(), req.GetNumber())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &parkingv1.SpotResponse{Spot: toProtoSpot(spot)}, nil
}

func (h *ParkingGRPCHandler) GetSpot(ctx context.Context, req *parkingv1.GetSpotRequest) (*parkingv1.SpotResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "spot id is required")
	}

	spot, err := h.spotUC.GetSpot(req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "spot not found")
	}

	return &parkingv1.SpotResponse{Spot: toProtoSpot(spot)}, nil
}

func (h *ParkingGRPCHandler) GetSpotsByParking(ctx context.Context, req *parkingv1.GetSpotsByParkingRequest) (*parkingv1.SpotListResponse, error) {
	if req.GetParkingId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking_id is required")
	}

	spots, err := h.spotUC.GetSpotsByParking(req.GetParkingId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	result := make([]*parkingv1.Spot, 0, len(spots))
	for _, spot := range spots {
		s := spot
		result = append(result, toProtoSpot(&s))
	}

	return &parkingv1.SpotListResponse{Spots: result}, nil
}

func (h *ParkingGRPCHandler) UpdateSpotStatus(ctx context.Context, req *parkingv1.UpdateSpotStatusRequest) (*parkingv1.ActionResponse, error) {
	if req.GetSpotId() <= 0 || req.GetStatus() == "" {
		return nil, status.Error(codes.InvalidArgument, "spot_id and status are required")
	}

	if err := h.spotUC.UpdateSpotStatus(req.GetSpotId(), domain.SpotStatus(req.GetStatus())); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return actionOK("spot status updated"), nil
}

func (h *ParkingGRPCHandler) ReserveSpot(ctx context.Context, req *parkingv1.SpotActionRequest) (*parkingv1.ActionResponse, error) {
	if req.GetSpotId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "spot_id is required")
	}

	if err := h.spotUC.ReserveSpot(req.GetSpotId()); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return actionOK("spot reserved"), nil
}

func (h *ParkingGRPCHandler) ReleaseSpot(ctx context.Context, req *parkingv1.SpotActionRequest) (*parkingv1.ActionResponse, error) {
	if req.GetSpotId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "spot_id is required")
	}

	if err := h.spotUC.ReleaseSpot(req.GetSpotId()); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return actionOK("spot released"), nil
}

func (h *ParkingGRPCHandler) DeleteSpot(ctx context.Context, req *parkingv1.DeleteSpotRequest) (*parkingv1.ActionResponse, error) {
	if req.GetSpotId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "spot_id is required")
	}

	if err := h.spotUC.DeleteSpot(req.GetSpotId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return actionOK("spot deleted"), nil
}

func (h *ParkingGRPCHandler) CreateTariff(ctx context.Context, req *parkingv1.CreateTariffRequest) (*parkingv1.TariffResponse, error) {
	if req.GetParkingId() <= 0 || req.GetPricePerHour() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking_id and price_per_hour are required")
	}

	tariff, err := h.tariffUC.CreateTariff(req.GetParkingId(), req.GetPricePerHour())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &parkingv1.TariffResponse{Tariff: toProtoTariff(tariff)}, nil
}

func (h *ParkingGRPCHandler) GetTariff(ctx context.Context, req *parkingv1.GetTariffRequest) (*parkingv1.TariffResponse, error) {
	if req.GetParkingId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking_id is required")
	}

	tariff, err := h.tariffUC.GetTariff(req.GetParkingId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "tariff not found")
	}

	return &parkingv1.TariffResponse{Tariff: toProtoTariff(tariff)}, nil
}

func (h *ParkingGRPCHandler) UpdateTariff(ctx context.Context, req *parkingv1.UpdateTariffRequest) (*parkingv1.ActionResponse, error) {
	if req.GetParkingId() <= 0 || req.GetPricePerHour() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking_id and price_per_hour are required")
	}

	if err := h.tariffUC.UpdateTariff(req.GetParkingId(), req.GetPricePerHour()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return actionOK("tariff updated"), nil
}

func (h *ParkingGRPCHandler) CalculatePrice(ctx context.Context, req *parkingv1.CalculatePriceRequest) (*parkingv1.CalculatePriceResponse, error) {
	if req.GetParkingId() <= 0 || req.GetHours() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "parking_id and hours are required")
	}

	price, err := h.tariffUC.CalculatePrice(req.GetParkingId(), req.GetHours())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &parkingv1.CalculatePriceResponse{TotalPrice: price}, nil
}

func toProtoParking(parking *domain.Parking) *parkingv1.Parking {
	if parking == nil {
		return nil
	}

	return &parkingv1.Parking{
		Id:         parking.ID,
		Name:       parking.Name,
		Address:    parking.Address,
		TotalSpots: int32(parking.TotalSpots),
		CreatedAt:  parking.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toProtoSpot(spot *domain.Spot) *parkingv1.Spot {
	if spot == nil {
		return nil
	}

	return &parkingv1.Spot{
		Id:        spot.ID,
		ParkingId: spot.ParkingID,
		Number:    spot.Number,
		Status:    string(spot.Status),
		CreatedAt: spot.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toProtoTariff(tariff *domain.Tariff) *parkingv1.Tariff {
	if tariff == nil {
		return nil
	}

	return &parkingv1.Tariff{
		Id:           tariff.ID,
		ParkingId:    tariff.ParkingID,
		PricePerHour: tariff.PricePerHour,
		CreatedAt:    tariff.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func actionOK(message string) *parkingv1.ActionResponse {
	return &parkingv1.ActionResponse{
		Success: true,
		Message: message,
	}
}
