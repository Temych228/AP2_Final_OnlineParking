package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/service"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/transport/http"
	"github.com/gin-gonic/gin"
)

type fakeUserRepo struct {
	users map[string]*domain.User
	next  int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]*domain.User), next: 1}
}

func cloneUser(u *domain.User) *domain.User {
	if u == nil {
		return nil
	}
	copy := *u
	return &copy
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := r.users[strings.TrimSpace(id)]; ok {
		return cloneUser(u), nil
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, u := range r.users {
		if u.Email == email {
			return cloneUser(u), nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) Update(_ context.Context, id string, input domain.UpdateInput) (*domain.User, error) {
	u, ok := r.users[strings.TrimSpace(id)]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	u.FirstName = strings.TrimSpace(input.FirstName)
	u.LastName = strings.TrimSpace(input.LastName)
	u.Phone = strings.TrimSpace(input.Phone)
	u.UpdatedAt = time.Now().UTC()
	return cloneUser(u), nil
}

func (r *fakeUserRepo) Delete(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *fakeUserRepo) List(_ context.Context, page, pageSize int, role string) ([]*domain.User, int, error) {
	items := make([]*domain.User, 0)
	for _, u := range r.users {
		if role == "" || string(u.Role) == role {
			items = append(items, cloneUser(u))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, len(items), nil
}

func (r *fakeUserRepo) Create(_ context.Context, input domain.CreateInput) (*domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	for _, u := range r.users {
		if u.Email == email {
			return nil, domain.ErrEmailTaken
		}
	}
	id := fmt.Sprintf("user-%d", r.next)
	r.next++

	u := &domain.User{
		ID:        id,
		Email:     email,
		FirstName: strings.TrimSpace(input.FirstName),
		LastName:  strings.TrimSpace(input.LastName),
		Phone:     strings.TrimSpace(input.Phone),
		Role:      input.Role,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	r.users[id] = u
	return cloneUser(u), nil
}

func (r *fakeUserRepo) GetBatch(_ context.Context, ids []string) ([]*domain.User, error) {
	out := make([]*domain.User, 0, len(ids))
	for _, id := range ids {
		if u, ok := r.users[strings.TrimSpace(id)]; ok {
			out = append(out, cloneUser(u))
		}
	}
	return out, nil
}

func (r *fakeUserRepo) CheckExists(_ context.Context, email string) (bool, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, u := range r.users {
		if u.Email == email {
			return true, u.ID, nil
		}
	}
	return false, "", nil
}

func (r *fakeUserRepo) VerifyEmail(_ context.Context, userID string) error {
	u, ok := r.users[strings.TrimSpace(userID)]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.IsVerified = true
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *fakeUserRepo) Ban(_ context.Context, userID, reason string) error {
	u, ok := r.users[strings.TrimSpace(userID)]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.IsBanned = true
	u.BanReason = strings.TrimSpace(reason)
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *fakeUserRepo) CreateWithID(_ context.Context, id string, input domain.CreateInput) (*domain.User, error) {
	id = strings.TrimSpace(id)
	if _, ok := r.users[id]; ok {
		return nil, domain.ErrEmailTaken
	}
	u := &domain.User{
		ID:        id,
		Email:     strings.ToLower(strings.TrimSpace(input.Email)),
		FirstName: strings.TrimSpace(input.FirstName),
		LastName:  strings.TrimSpace(input.LastName),
		Phone:     strings.TrimSpace(input.Phone),
		Role:      input.Role,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	r.users[id] = u
	return cloneUser(u), nil
}

func newUserService() (*service.UserService, *fakeUserRepo) {
	repo := newFakeUserRepo()
	return service.New(repo), repo
}

func TestUserService_Unit(t *testing.T) {
	svc, repo := newUserService()
	ctx := context.Background()

	user, err := svc.CreateUser(ctx, domain.CreateInput{
		Email:     "User@Mail.Com",
		FirstName: "Test",
		LastName:  "User",
		Phone:     "+77001112233",
		Role:      domain.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.ID == "" || user.Email != "user@mail.com" {
		t.Fatalf("unexpected user: %#v", user)
	}

	got, err := svc.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.Email != "user@mail.com" {
		t.Fatalf("unexpected get user: %#v", got)
	}

	updated, err := svc.UpdateUser(ctx, user.ID, domain.UpdateInput{
		FirstName: "Updated",
		LastName:  "User",
		Phone:     "+77001112234",
	})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.FirstName != "Updated" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}

	exists, userID, err := svc.CheckUserExists(ctx, "user@mail.com")
	if err != nil || !exists || userID != user.ID {
		t.Fatalf("unexpected exists response: exists=%v userID=%s err=%v", exists, userID, err)
	}

	if err := svc.VerifyUserEmail(ctx, user.ID); err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if u := repo.users[user.ID]; u == nil || !u.IsVerified {
		t.Fatal("expected verified user")
	}

	if err := svc.BanUser(ctx, user.ID, "spam"); err != nil {
		t.Fatalf("ban user: %v", err)
	}
	if u := repo.users[user.ID]; u == nil || !u.IsBanned || u.BanReason != "spam" {
		t.Fatalf("unexpected banned user: %#v", u)
	}

	list, total, err := svc.ListUsers(ctx, 1, 20, "")
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("unexpected list result: total=%d len=%d err=%v", total, len(list), err)
	}

	batch, err := svc.GetUsersBatch(ctx, []string{user.ID})
	if err != nil || len(batch) != 1 {
		t.Fatalf("unexpected batch result: len=%d err=%v", len(batch), err)
	}

	bookings, points, balance, err := svc.GetUserStats(ctx, user.ID)
	if err != nil || bookings != 0 || points != 0 || balance != 0 {
		t.Fatalf("unexpected stats: %d %d %f err=%v", bookings, points, balance, err)
	}

	if err := svc.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := svc.GetUser(ctx, user.ID); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestUserService_Errors(t *testing.T) {
	svc, _ := newUserService()
	ctx := context.Background()

	if _, err := svc.CreateUser(ctx, domain.CreateInput{Email: "bad"}); err == nil {
		t.Fatal("expected invalid input")
	}

	if _, err := svc.UpdateUser(ctx, "missing", domain.UpdateInput{FirstName: "A", LastName: "B"}); err == nil {
		t.Fatal("expected update error")
	}

	if err := svc.DeleteUser(ctx, "missing"); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestUserHTTP_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newUserService()
	handler := httptransport.New(svc)
	router := gin.New()
	handler.Register(router)

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/users", mustJSON(t, map[string]any{
		"email":      "frontend@mail.com",
		"first_name": "Front",
		"last_name":  "End",
		"phone":      "+77001112233",
		"role":       "user",
	})))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected id in response: %s", createResp.Body.String())
	}

	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
	}

	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, httptest.NewRequest(http.MethodPut, "/users/"+id, mustJSON(t, map[string]any{
		"first_name": "Updated",
		"last_name":  "User",
		"phone":      "+77001112234",
	})))
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/users?page=1&page_size=20", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
	}

	checkResp := httptest.NewRecorder()
	router.ServeHTTP(checkResp, httptest.NewRequest(http.MethodGet, "/users/check?email=frontend@mail.com", nil))
	if checkResp.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", checkResp.Code, checkResp.Body.String())
	}

	verifyResp := httptest.NewRecorder()
	router.ServeHTTP(verifyResp, httptest.NewRequest(http.MethodPost, "/users/"+id+"/verify", nil))
	if verifyResp.Code != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verifyResp.Code, verifyResp.Body.String())
	}

	banResp := httptest.NewRecorder()
	router.ServeHTTP(banResp, httptest.NewRequest(http.MethodPost, "/users/"+id+"/ban", mustJSON(t, map[string]any{
		"reason": "spam",
	})))
	if banResp.Code != http.StatusOK {
		t.Fatalf("ban status = %d body=%s", banResp.Code, banResp.Body.String())
	}

	statsResp := httptest.NewRecorder()
	router.ServeHTTP(statsResp, httptest.NewRequest(http.MethodGet, "/users/"+id+"/stats", nil))
	if statsResp.Code != http.StatusOK {
		t.Fatalf("stats status = %d body=%s", statsResp.Code, statsResp.Body.String())
	}

	batchResp := httptest.NewRecorder()
	router.ServeHTTP(batchResp, httptest.NewRequest(http.MethodPost, "/users/batch", mustJSON(t, map[string]any{
		"ids": []string{id},
	})))
	if batchResp.Code != http.StatusOK {
		t.Fatalf("batch status = %d body=%s", batchResp.Code, batchResp.Body.String())
	}

	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, httptest.NewRequest(http.MethodDelete, "/users/"+id, nil))
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(data)
}
