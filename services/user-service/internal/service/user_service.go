package service

import (
	"context"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/repository"
)

type UserService struct {
	repo *repository.UserRepository
}

func New(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UserService) UpdateUser(ctx context.Context, id string, input domain.UpdateInput) (*domain.User, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, input)
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserService) ListUsers(ctx context.Context, page, pageSize int, role string) ([]*domain.User, int, error) {
	role = strings.TrimSpace(role)
	return s.repo.List(ctx, page, pageSize, role)
}

func (s *UserService) CreateUser(ctx context.Context, input domain.CreateInput) (*domain.User, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, input)
}

func (s *UserService) GetUsersBatch(ctx context.Context, ids []string) ([]*domain.User, error) {
	return s.repo.GetBatch(ctx, ids)
}

func (s *UserService) CheckUserExists(ctx context.Context, email string) (bool, string, error) {
	return s.repo.CheckExists(ctx, email)
}

func (s *UserService) VerifyUserEmail(ctx context.Context, userID string) error {
	return s.repo.VerifyEmail(ctx, userID)
}

func (s *UserService) BanUser(ctx context.Context, userID, reason string) error {
	reason = strings.TrimSpace(reason)
	return s.repo.Ban(ctx, userID, reason)
}

func (s *UserService) GetUserStats(ctx context.Context, userID string) (int32, int32, float32, error) {
	if _, err := s.repo.GetByID(ctx, userID); err != nil {
		return 0, 0, 0, err
	}
	return 0, 0, 0, nil
}

func (s *UserService) CreateUserWithID(ctx context.Context, id string, input domain.CreateInput) (*domain.User, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return s.repo.CreateWithID(ctx, id, input)
}
