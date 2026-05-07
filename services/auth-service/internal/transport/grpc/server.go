package grpcserver

import (
	"context"
	"errors"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/service"
	authv1 "github.com/Temych228/ap2_protos_go_final/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	authv1.UnimplementedAuthServiceServer
	svc *service.AuthService
}

func New(svc *service.AuthService) *Server {
	return &Server{svc: svc}
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	userID, err := s.svc.Register(ctx, domain.RegisterInput{
		Email:     req.GetEmail(),
		Password:  req.GetPassword(),
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Phone:     req.GetPhone(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &authv1.RegisterResponse{
		UserId:  userID,
		Message: "Verification email sent",
	}, nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	tokens, err := s.svc.Login(ctx, domain.LoginInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &authv1.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    timestamppb.New(tokens.ExpiresAt),
		UserId:       tokens.UserID,
	}, nil
}

func (s *Server) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	tokens, err := s.svc.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, mapError(err)
	}

	return &authv1.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    timestamppb.New(tokens.ExpiresAt),
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := s.svc.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.LogoutResponse{Success: true}, nil
}

func (s *Server) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	userID, email, err := s.svc.ValidateToken(req.GetAccessToken())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.ValidateTokenResponse{
		Valid:  true,
		UserId: userID,
		Email:  email,
		Role:   "user",
	}, nil
}

func (s *Server) VerifyEmail(ctx context.Context, req *authv1.VerifyEmailRequest) (*authv1.VerifyEmailResponse, error) {
	if err := s.svc.VerifyEmail(ctx, req.GetToken()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.VerifyEmailResponse{
		Success: true,
		Message: "Email verified",
	}, nil
}

func (s *Server) ForgotPassword(ctx context.Context, req *authv1.ForgotPasswordRequest) (*authv1.ForgotPasswordResponse, error) {
	token, err := s.svc.ForgotPassword(ctx, req.GetEmail())
	if err != nil {
		return nil, mapError(err)
	}
	_ = token
	return &authv1.ForgotPasswordResponse{
		Success: true,
		Message: "Password reset token created",
	}, nil
}

func (s *Server) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	if err := s.svc.ResetPassword(ctx, req.GetToken(), req.GetNewPassword()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.ResetPasswordResponse{Success: true}, nil
}

func (s *Server) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	if err := s.svc.ChangePassword(ctx, req.GetUserId(), req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.ChangePasswordResponse{Success: true}, nil
}

func (s *Server) GetSession(ctx context.Context, req *authv1.GetSessionRequest) (*authv1.GetSessionResponse, error) {
	userID, email, expiresAt, err := s.svc.GetSession(ctx, req.GetAccessToken())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.GetSessionResponse{
		UserId:    userID,
		Email:     email,
		Role:      "user",
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

func (s *Server) RevokeAllSessions(ctx context.Context, req *authv1.RevokeAllSessionsRequest) (*authv1.RevokeAllSessionsResponse, error) {
	count, err := s.svc.RevokeAllSessions(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.RevokeAllSessionsResponse{RevokedCount: count}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrTokenNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
