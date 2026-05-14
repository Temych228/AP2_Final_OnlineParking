package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *service.UserService
}

func New(svc *service.UserService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(router *gin.Engine) {
	router.GET("/health", h.Health)
	router.GET("/ready", h.Health)

	users := router.Group("/users")
	{
		users.GET("", h.ListUsers)
		users.GET("/", h.ListUsers)
		users.POST("", h.CreateUser)
		users.POST("/", h.CreateUser)
		users.GET("/:id", h.GetUser)
		users.PUT("/:id", h.UpdateUser)
		users.DELETE("/:id", h.DeleteUser)
		users.POST("/:id/verify", h.VerifyUserEmail)
		users.POST("/:id/ban", h.BanUser)
		users.GET("/:id/stats", h.GetUserStats)
		users.POST("/batch", h.GetUsersBatch)
		users.GET("/check", h.CheckUserExists)
	}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) GetUser(c *gin.Context) {
	user, err := h.svc.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, err := h.svc.CreateUser(c.Request.Context(), domain.CreateInput{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Role:      domain.Role(req.Role),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(user))
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, err := h.svc.UpdateUser(c.Request.Context(), c.Param("id"), domain.UpdateInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) DeleteUser(c *gin.Context) {
	if err := h.svc.DeleteUser(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	role := strings.TrimSpace(c.Query("role"))

	users, total, err := h.svc.ListUsers(c.Request.Context(), page, pageSize, role)
	if err != nil {
		h.writeError(c, err)
		return
	}

	items := make([]userResponse, 0, len(users))
	for _, u := range users {
		items = append(items, toUserResponse(u))
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"role":      role,
	})
}

func (h *Handler) GetUsersBatch(c *gin.Context) {
	var req batchUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	users, err := h.svc.GetUsersBatch(c.Request.Context(), req.IDs)
	if err != nil {
		h.writeError(c, err)
		return
	}

	items := make([]userResponse, 0, len(users))
	for _, u := range users {
		items = append(items, toUserResponse(u))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CheckUserExists(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	exists, userID, err := h.svc.CheckUserExists(c.Request.Context(), email)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exists":  exists,
		"user_id": userID,
	})
}

func (h *Handler) VerifyUserEmail(c *gin.Context) {
	if err := h.svc.VerifyUserEmail(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) BanUser(c *gin.Context) {
	var req banUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.svc.BanUser(c.Request.Context(), c.Param("id"), req.Reason); err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) GetUserStats(c *gin.Context) {
	bookings, points, balance, err := h.svc.GetUserStats(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  c.Param("id"),
		"bookings": bookings,
		"points":   points,
		"balance":  balance,
	})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrUserBanned):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func toUserResponse(u *domain.User) userResponse {
	if u == nil {
		return userResponse{}
	}

	return userResponse{
		ID:         u.ID,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Phone:      u.Phone,
		Role:       string(u.Role),
		IsVerified: u.IsVerified,
		IsBanned:   u.IsBanned,
		BanReason:  u.BanReason,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}

type createUserRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
}

type updateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type batchUsersRequest struct {
	IDs []string `json:"ids"`
}

type banUserRequest struct {
	Reason string `json:"reason"`
}

type userResponse struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Phone      string    `json:"phone"`
	Role       string    `json:"role"`
	IsVerified bool      `json:"is_verified"`
	IsBanned   bool      `json:"is_banned"`
	BanReason  string    `json:"ban_reason"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
