package grpcserver

import (
	"context"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/service"
	notificationv1 "github.com/Temych228/ap2_protos_go_final/notification/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	notificationv1.UnimplementedNotificationServiceServer
	svc *service.NotificationService
}

func New(svc *service.NotificationService) *Server {
	return &Server{svc: svc}
}

func (s *Server) SendEmail(ctx context.Context, req *notificationv1.SendEmailRequest) (*notificationv1.SendEmailResponse, error) {
	n, err := s.svc.SendEmail(ctx, req.GetTo(), req.GetSubject(), req.GetBody(), req.GetUserId(), req.GetType())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.SendEmailResponse{
		Success:        n != nil && n.Status == domain.StatusSent,
		NotificationId: n.ID,
	}, nil
}

func (s *Server) SendBulkEmail(ctx context.Context, req *notificationv1.SendBulkEmailRequest) (*notificationv1.SendBulkEmailResponse, error) {
	sent, failed, err := s.svc.SendBulkEmail(ctx, req.GetTo(), req.GetSubject(), req.GetBody(), req.GetType())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.SendBulkEmailResponse{
		SentCount:   sent,
		FailedCount: failed,
	}, nil
}

func (s *Server) GetNotificationHistory(ctx context.Context, req *notificationv1.GetNotificationHistoryRequest) (*notificationv1.GetNotificationHistoryResponse, error) {
	items, total, err := s.svc.GetNotificationHistory(ctx, req.GetUserId(), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*notificationv1.Notification, 0, len(items))
	for _, n := range items {
		out = append(out, toProtoNotification(n))
	}

	return &notificationv1.GetNotificationHistoryResponse{
		Notifications: out,
		Total:         int32(total),
	}, nil
}

func (s *Server) MarkNotificationRead(ctx context.Context, req *notificationv1.MarkNotificationReadRequest) (*notificationv1.MarkNotificationReadResponse, error) {
	if err := s.svc.MarkNotificationRead(ctx, req.GetNotificationId(), req.GetUserId()); err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.MarkNotificationReadResponse{Success: true}, nil
}

func (s *Server) StreamNotifications(req *notificationv1.StreamNotificationsRequest, stream notificationv1.NotificationService_StreamNotificationsServer) error {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}

	ctx := stream.Context()
	ch := s.svc.Subscribe(userID)
	defer s.svc.Unsubscribe(userID, ch)

	history, _, err := s.svc.GetNotificationHistory(ctx, userID, 1, 50)
	if err == nil {
		for _, n := range history {
			_ = stream.Send(&notificationv1.NotificationEvent{
				EventType:    "snapshot",
				Notification: toProtoNotification(n),
			})
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case n, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(&notificationv1.NotificationEvent{
				EventType:    "new",
				Notification: toProtoNotification(n),
			}); err != nil {
				return status.Error(codes.Internal, err.Error())
			}
		}
	}
}

func (s *Server) GetUnreadCount(ctx context.Context, req *notificationv1.GetUnreadCountRequest) (*notificationv1.GetUnreadCountResponse, error) {
	count, err := s.svc.GetUnreadCount(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.GetUnreadCountResponse{Count: count}, nil
}

func (s *Server) DeleteNotification(ctx context.Context, req *notificationv1.DeleteNotificationRequest) (*notificationv1.DeleteNotificationResponse, error) {
	if err := s.svc.DeleteNotification(ctx, req.GetNotificationId(), req.GetUserId()); err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.DeleteNotificationResponse{Success: true}, nil
}

func (s *Server) SendSMS(ctx context.Context, req *notificationv1.SendSMSRequest) (*notificationv1.SendSMSResponse, error) {
	n, err := s.svc.SendSMS(ctx, req.GetPhone(), req.GetMessage(), req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.SendSMSResponse{
		Success: n != nil && n.Status == domain.StatusSent,
		Message: "sent",
	}, nil
}

func (s *Server) GetTemplate(ctx context.Context, req *notificationv1.GetTemplateRequest) (*notificationv1.GetTemplateResponse, error) {
	subject, body := s.svc.GetTemplate(req.GetTemplateName())
	return &notificationv1.GetTemplateResponse{
		Subject: subject,
		Body:    body,
	}, nil
}

func (s *Server) UpdatePreferences(ctx context.Context, req *notificationv1.UpdatePreferencesRequest) (*notificationv1.UpdatePreferencesResponse, error) {
	if req.GetPreferences() == nil {
		return nil, status.Error(codes.InvalidArgument, "preferences are required")
	}

	err := s.svc.UpdatePreferences(ctx, &domain.Preferences{
		UserID:         req.GetPreferences().GetUserId(),
		EmailEnabled:   req.GetPreferences().GetEmailEnabled(),
		SMSEnabled:     req.GetPreferences().GetSmsEnabled(),
		PushEnabled:    req.GetPreferences().GetPushEnabled(),
		MarketingEmail: req.GetPreferences().GetMarketingEmails(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.UpdatePreferencesResponse{Success: true}, nil
}

func (s *Server) GetPreferences(ctx context.Context, req *notificationv1.GetPreferencesRequest) (*notificationv1.GetPreferencesResponse, error) {
	p, err := s.svc.GetPreferences(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.GetPreferencesResponse{
		Preferences: &notificationv1.NotificationPreferences{
			UserId:          p.UserID,
			EmailEnabled:    p.EmailEnabled,
			SmsEnabled:      p.SMSEnabled,
			PushEnabled:     p.PushEnabled,
			MarketingEmails: p.MarketingEmail,
		},
	}, nil
}

func (s *Server) SendPush(ctx context.Context, req *notificationv1.SendPushRequest) (*notificationv1.SendPushResponse, error) {
	n, err := s.svc.SendPush(ctx, req.GetUserId(), req.GetTitle(), req.GetBody(), req.GetData())
	if err != nil {
		return nil, mapError(err)
	}

	return &notificationv1.SendPushResponse{
		Success: n != nil && n.Status == domain.StatusSent,
	}, nil
}

func toProtoNotification(n *domain.Notification) *notificationv1.Notification {
	if n == nil {
		return nil
	}

	var sentAt *timestamppb.Timestamp
	if n.SentAt != nil {
		sentAt = timestamppb.New(*n.SentAt)
	}

	return &notificationv1.Notification{
		Id:        n.ID,
		UserId:    n.UserID,
		Type:      string(n.Type),
		Subject:   n.Subject,
		Body:      n.Body,
		IsRead:    n.IsRead,
		Status:    string(n.Status),
		CreatedAt: timestamppb.New(n.CreatedAt),
		SentAt:    sentAt,
	}
}

func mapError(err error) error {
	switch {
	case strings.Contains(err.Error(), domain.ErrNotFound.Error()):
		return status.Error(codes.NotFound, err.Error())
	case strings.Contains(err.Error(), domain.ErrInvalidInput.Error()):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
