package grpcserver

import (
	"context"
	"errors"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	bookingv1 "github.com/Temych228/ap2_protos_go_final/booking/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	bookingv1.UnimplementedBookingServiceServer
	svc *service.BookingService
}

func New(svc *service.BookingService) *Server {
	return &Server{svc: svc}
}

func (s *Server) CreateBooking(ctx context.Context, req *bookingv1.CreateBookingRequest) (*bookingv1.CreateBookingResponse, error) {
	booking, err := s.svc.CreateBooking(ctx, domain.CreateInput{
		UserID:       req.GetUserId(),
		ParkingID:    req.GetParkingId(),
		SpotID:       req.GetSpotId(),
		VehiclePlate: req.GetVehiclePlate(),
		StartTime:    req.GetStartTime().AsTime(),
		EndTime:      req.GetEndTime().AsTime(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &bookingv1.CreateBookingResponse{
		BookingId: booking.ID,
		Status:    string(booking.Status),
	}, nil
}

func (s *Server) GetBooking(ctx context.Context, req *bookingv1.GetBookingRequest) (*bookingv1.GetBookingResponse, error) {
	booking, err := s.svc.GetBooking(ctx, req.GetBookingId())
	if err != nil {
		return nil, mapError(err)
	}

	return mapBookingToGetResponse(booking), nil
}

func (s *Server) ListBookings(ctx context.Context, req *bookingv1.ListBookingsRequest) (*bookingv1.ListBookingsResponse, error) {
	items, total, err := s.svc.ListBookings(ctx, domain.ListFilter{
		Page:      int(req.GetPage()),
		PageSize:  int(req.GetPageSize()),
		UserID:    req.GetUserId(),
		ParkingID: req.GetParkingId(),
		SpotID:    req.GetSpotId(),
		Status:    domain.BookingStatus(req.GetStatus()),
	})
	if err != nil {
		return nil, mapError(err)
	}

	resp := &bookingv1.ListBookingsResponse{
		Total: int32(total),
		Items: make([]*bookingv1.BookingItem, 0, len(items)),
	}

	for _, b := range items {
		resp.Items = append(resp.Items, mapBookingToItem(b))
	}

	return resp, nil
}

func (s *Server) ConfirmBooking(ctx context.Context, req *bookingv1.ConfirmBookingRequest) (*bookingv1.ConfirmBookingResponse, error) {
	booking, err := s.svc.ConfirmBooking(ctx, req.GetBookingId())
	if err != nil {
		return nil, mapError(err)
	}

	return &bookingv1.ConfirmBookingResponse{
		BookingId: booking.ID,
		Status:    string(booking.Status),
	}, nil
}

func (s *Server) CancelBooking(ctx context.Context, req *bookingv1.CancelBookingRequest) (*bookingv1.CancelBookingResponse, error) {
	booking, err := s.svc.CancelBooking(ctx, req.GetBookingId(), req.GetReason())
	if err != nil {
		return nil, mapError(err)
	}

	return &bookingv1.CancelBookingResponse{
		BookingId: booking.ID,
		Status:    string(booking.Status),
	}, nil
}

func (s *Server) StartBooking(ctx context.Context, req *bookingv1.StartBookingRequest) (*bookingv1.StartBookingResponse, error) {
	booking, err := s.svc.StartBooking(ctx, req.GetBookingId())
	if err != nil {
		return nil, mapError(err)
	}

	return &bookingv1.StartBookingResponse{
		BookingId: booking.ID,
		Status:    string(booking.Status),
	}, nil
}

func (s *Server) CompleteBooking(ctx context.Context, req *bookingv1.CompleteBookingRequest) (*bookingv1.CompleteBookingResponse, error) {
	booking, err := s.svc.CompleteBooking(ctx, req.GetBookingId())
	if err != nil {
		return nil, mapError(err)
	}

	return &bookingv1.CompleteBookingResponse{
		BookingId: booking.ID,
		Status:    string(booking.Status),
	}, nil
}

func mapBookingToGetResponse(b *domain.Booking) *bookingv1.GetBookingResponse {
	var cancelledAt *timestamppb.Timestamp
	if b.CancelledAt != nil {
		cancelledAt = timestamppb.New(*b.CancelledAt)
	}

	return &bookingv1.GetBookingResponse{
		BookingId:    b.ID,
		UserId:       b.UserID,
		ParkingId:    b.ParkingID,
		SpotId:       b.SpotID,
		VehiclePlate: b.VehiclePlate,
		StartTime:    timestamppb.New(b.StartTime),
		EndTime:      timestamppb.New(b.EndTime),
		Status:       string(b.Status),
		CancelReason: b.CancelReason,
		CreatedAt:    timestamppb.New(b.CreatedAt),
		UpdatedAt:    timestamppb.New(b.UpdatedAt),
		CancelledAt:  cancelledAt,
	}
}

func mapBookingToItem(b *domain.Booking) *bookingv1.BookingItem {
	return &bookingv1.BookingItem{
		BookingId: b.ID,
		UserId:    b.UserID,
		ParkingId: b.ParkingID,
		SpotId:    b.SpotID,
		Status:    string(b.Status),
		StartTime: timestamppb.New(b.StartTime),
		EndTime:   timestamppb.New(b.EndTime),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrBookingConflict),
		errors.Is(err, domain.ErrInvalidStatusTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrUserNotFound),
		errors.Is(err, domain.ErrParkingLotNotFound),
		errors.Is(err, domain.ErrParkingSpotNotFound):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
