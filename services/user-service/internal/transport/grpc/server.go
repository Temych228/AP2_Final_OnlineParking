package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/service"
	userv1 "github.com/Temych228/ap2_protos_go_final/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	userv1.UnimplementedUserServiceServer
	svc *service.UserService
}

func New(svc *service.UserService) *Server {
	return &Server{svc: svc}
}

func (s *Server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := s.svc.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.GetUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) GetUserByEmail(ctx context.Context, req *userv1.GetUserByEmailRequest) (*userv1.GetUserResponse, error) {
	if strings.TrimSpace(req.GetEmail()) == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	user, err := s.svc.GetUserByEmail(ctx, req.GetEmail())
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.GetUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := s.svc.UpdateUser(ctx, req.GetUserId(), domain.UpdateInput{
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Phone:     req.GetPhone(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.UpdateUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := s.svc.DeleteUser(ctx, req.GetUserId()); err != nil {
		return nil, mapError(err)
	}

	return &userv1.DeleteUserResponse{Success: true}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	users, total, err := s.svc.ListUsers(ctx, int(req.GetPage()), int(req.GetPageSize()), req.GetRole())
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*userv1.User, 0, len(users))
	for _, u := range users {
		out = append(out, toProtoUser(u))
	}

	return &userv1.ListUsersResponse{
		Users: out,
		Total: int32(total),
	}, nil
}

func (s *Server) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := s.svc.CreateUser(ctx, domain.CreateInput{
		Email:     req.GetEmail(),
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Phone:     req.GetPhone(),
		Role:      domain.Role(req.GetRole()),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.CreateUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) GetUsersBatch(ctx context.Context, req *userv1.GetUsersBatchRequest) (*userv1.GetUsersBatchResponse, error) {
	users, err := s.svc.GetUsersBatch(ctx, req.GetUserIds())
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*userv1.User, 0, len(users))
	for _, u := range users {
		out = append(out, toProtoUser(u))
	}

	return &userv1.GetUsersBatchResponse{Users: out}, nil
}

func (s *Server) CheckUserExists(ctx context.Context, req *userv1.CheckUserExistsRequest) (*userv1.CheckUserExistsResponse, error) {
	exists, userID, err := s.svc.CheckUserExists(ctx, req.GetEmail())
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.CheckUserExistsResponse{
		Exists: exists,
		UserId: userID,
	}, nil
}

func (s *Server) VerifyUserEmail(ctx context.Context, req *userv1.VerifyUserEmailRequest) (*userv1.VerifyUserEmailResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := s.svc.VerifyUserEmail(ctx, req.GetUserId()); err != nil {
		return nil, mapError(err)
	}

	return &userv1.VerifyUserEmailResponse{Success: true}, nil
}

func (s *Server) WatchUser(req *userv1.WatchUserRequest, stream userv1.UserService_WatchUserServer) error {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}

	ctx := stream.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var previous *domain.User

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			user, err := s.svc.GetUser(ctx, userID)
			if err != nil {
				return mapError(err)
			}

			if previous != nil && reflect.DeepEqual(previous, user) {
				continue
			}

			eventType := "snapshot"
			if previous != nil {
				eventType = "updated"
			}

			if err := stream.Send(&userv1.UserEvent{
				EventType: eventType,
				User:      toProtoUser(user),
			}); err != nil {
				return status.Error(codes.Internal, err.Error())
			}

			previous = user
		}
	}
}

func (s *Server) GetUserStats(ctx context.Context, req *userv1.GetUserStatsRequest) (*userv1.GetUserStatsResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if _, err := s.svc.GetUser(ctx, req.GetUserId()); err != nil {
		return nil, mapError(err)
	}

	totalBookings, activeBookings, totalSpent, err := s.svc.GetUserStats(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &userv1.GetUserStatsResponse{
		TotalBookings:  totalBookings,
		ActiveBookings: activeBookings,
		TotalSpent:     totalSpent,
	}, nil
}

func (s *Server) BanUser(ctx context.Context, req *userv1.BanUserRequest) (*userv1.BanUserResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := s.svc.BanUser(ctx, req.GetUserId(), req.GetReason()); err != nil {
		return nil, mapError(err)
	}

	return &userv1.BanUserResponse{Success: true}, nil
}

func toProtoUser(u *domain.User) *userv1.User {
	if u == nil {
		return nil
	}

	return &userv1.User{
		Id:         u.ID,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Phone:      u.Phone,
		Role:       string(u.Role),
		IsVerified: u.IsVerified,
		IsBanned:   u.IsBanned,
		CreatedAt:  timestamppb.New(u.CreatedAt),
		UpdatedAt:  timestamppb.New(u.UpdatedAt),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUserBanned):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("%v", err))
	}
}
