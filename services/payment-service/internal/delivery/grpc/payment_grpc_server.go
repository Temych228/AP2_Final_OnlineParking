package grpc

import (
	"context"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/service"

	paymentv1 "github.com/Temych228/ap2_protos_go_final/payment/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
	svc *service.PaymentService
}

func New(svc *service.PaymentService) *Server {
	return &Server{svc: svc}
}

func (s *Server) CreatePayment(ctx context.Context, req *paymentv1.CreatePaymentRequest) (*paymentv1.PaymentResponse, error) {
	payment, err := s.svc.CreatePayment(ctx, domain.CreatePaymentInput{
		BookingID: strings.TrimSpace(req.GetBookingId()),
		Method:    protoMethodToDomain(req.GetMethod()),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &paymentv1.PaymentResponse{
		Payment: toProtoPayment(payment),
	}, nil
}

func (s *Server) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.PaymentResponse, error) {
	payment, err := s.svc.GetPayment(ctx, strings.TrimSpace(req.GetId()))
	if err != nil {
		return nil, mapError(err)
	}

	return &paymentv1.PaymentResponse{
		Payment: toProtoPayment(payment),
	}, nil
}

func (s *Server) GetPaymentByBooking(ctx context.Context, req *paymentv1.GetPaymentByBookingRequest) (*paymentv1.PaymentResponse, error) {
	payment, err := s.svc.GetPaymentByBooking(ctx, strings.TrimSpace(req.GetBookingId()))
	if err != nil {
		return nil, mapError(err)
	}

	return &paymentv1.PaymentResponse{
		Payment: toProtoPayment(payment),
	}, nil
}

func (s *Server) ListPayments(ctx context.Context, req *paymentv1.ListPaymentsRequest) (*paymentv1.PaymentListResponse, error) {
	payments, err := s.svc.ListPayments(ctx, domain.ListPaymentsFilter{
		UserID: strings.TrimSpace(req.GetUserId()),
		Status: protoStatusToDomainString(req.GetStatus()),
	})
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*paymentv1.Payment, 0, len(payments))
	for i := range payments {
		out = append(out, toProtoPayment(&payments[i]))
	}

	return &paymentv1.PaymentListResponse{
		Payments: out,
	}, nil
}

func (s *Server) CancelPayment(ctx context.Context, req *paymentv1.CancelPaymentRequest) (*paymentv1.PaymentResponse, error) {
	payment, err := s.svc.CancelPayment(ctx, strings.TrimSpace(req.GetId()))
	if err != nil {
		return nil, mapError(err)
	}

	return &paymentv1.PaymentResponse{
		Payment: toProtoPayment(payment),
	}, nil
}

func (s *Server) Health(ctx context.Context, req *paymentv1.HealthRequest) (*paymentv1.HealthResponse, error) {
	return &paymentv1.HealthResponse{
		Service: "payment-service",
		Status:  "ok",
	}, nil
}

func toProtoPayment(p *domain.Payment) *paymentv1.Payment {
	if p == nil {
		return nil
	}

	var paidAt *timestamppb.Timestamp
	if p.PaidAt != nil {
		paidAt = timestamppb.New(*p.PaidAt)
	}

	return &paymentv1.Payment{
		Id:                p.ID,
		BookingId:         p.BookingID,
		UserId:            p.UserID,
		ParkingId:         p.ParkingID,
		SpotId:            p.SpotID,
		Amount:            p.Amount,
		Method:            domainMethodToProto(p.Method),
		Status:            domainStatusToProto(p.Status),
		ProviderPaymentId: p.ProviderPaymentID,
		FailureReason:     p.FailureReason,
		CreatedAt:         timestamppb.New(p.CreatedAt),
		PaidAt:            paidAt,
		UpdatedAt:         timestamppb.New(p.UpdatedAt),
	}
}

func protoMethodToDomain(method paymentv1.PaymentMethod) domain.PaymentMethod {
	switch method {
	case paymentv1.PaymentMethod_PAYMENT_METHOD_CARD:
		return domain.MethodCard
	case paymentv1.PaymentMethod_PAYMENT_METHOD_CASH:
		return domain.MethodCash
	case paymentv1.PaymentMethod_PAYMENT_METHOD_KASPI:
		return domain.MethodKaspi
	default:
		return ""
	}
}

func domainMethodToProto(method domain.PaymentMethod) paymentv1.PaymentMethod {
	switch method {
	case domain.MethodCard:
		return paymentv1.PaymentMethod_PAYMENT_METHOD_CARD
	case domain.MethodCash:
		return paymentv1.PaymentMethod_PAYMENT_METHOD_CASH
	case domain.MethodKaspi:
		return paymentv1.PaymentMethod_PAYMENT_METHOD_KASPI
	default:
		return paymentv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

func domainStatusToProto(status domain.PaymentStatus) paymentv1.PaymentStatus {
	switch status {
	case domain.StatusPending:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_PENDING
	case domain.StatusPaid:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_PAID
	case domain.StatusFailed:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_FAILED
	case domain.StatusCancelled:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_CANCELLED
	default:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomainString(status paymentv1.PaymentStatus) string {
	switch status {
	case paymentv1.PaymentStatus_PAYMENT_STATUS_PENDING:
		return string(domain.StatusPending)
	case paymentv1.PaymentStatus_PAYMENT_STATUS_PAID:
		return string(domain.StatusPaid)
	case paymentv1.PaymentStatus_PAYMENT_STATUS_FAILED:
		return string(domain.StatusFailed)
	case paymentv1.PaymentStatus_PAYMENT_STATUS_CANCELLED:
		return string(domain.StatusCancelled)
	default:
		return ""
	}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case strings.Contains(err.Error(), "not found"):
		return status.Error(codes.NotFound, err.Error())
	case strings.Contains(err.Error(), "required"):
		return status.Error(codes.InvalidArgument, err.Error())
	case strings.Contains(err.Error(), "invalid"):
		return status.Error(codes.InvalidArgument, err.Error())
	case strings.Contains(err.Error(), "already paid"):
		return status.Error(codes.FailedPrecondition, err.Error())
	case strings.Contains(err.Error(), "cannot be paid"):
		return status.Error(codes.FailedPrecondition, err.Error())
	case strings.Contains(err.Error(), "only pending payment can be cancelled"):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
