package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *service.BookingService
}

type createBookingRequest struct {
	UserID       string `json:"user_id"`
	ParkingID    int64  `json:"parking_id"`
	SpotID       int64  `json:"spot_id"`
	VehiclePlate string `json:"vehicle_plate"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
}

type cancelBookingRequest struct {
	Reason string `json:"reason"`
}

func New(service *service.BookingService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router *gin.Engine) {
	router.GET("/health", h.Health)
	router.GET("/ready", h.Health)

	bookings := router.Group("/bookings")
	bookings.POST("", h.CreateBooking)
	bookings.GET("", h.ListBookings)
	bookings.GET("/:id", h.GetBooking)
	bookings.POST("/:id/confirm", h.ConfirmBooking)
	bookings.POST("/:id/cancel", h.CancelBooking)
	bookings.POST("/:id/start", h.StartBooking)
	bookings.POST("/:id/complete", h.CompleteBooking)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) CreateBooking(c *gin.Context) {
	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartTime))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_time"})
		return
	}

	endTime, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndTime))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_time"})
		return
	}

	booking, err := h.service.CreateBooking(c.Request.Context(), domain.CreateInput{
		UserID:       req.UserID,
		ParkingID:    req.ParkingID,
		SpotID:       req.SpotID,
		VehiclePlate: req.VehiclePlate,
		StartTime:    startTime,
		EndTime:      endTime,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, booking)
}

func (h *Handler) GetBooking(c *gin.Context) {
	booking, err := h.service.GetBooking(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *Handler) ListBookings(c *gin.Context) {
	page, err := parseIntQuery(c, "page", 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
		return
	}

	pageSize, err := parseIntQuery(c, "page_size", 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page_size"})
		return
	}

	parkingID, err := parseInt64Query(c, "parking_id", 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking_id"})
		return
	}

	spotID, err := parseInt64Query(c, "spot_id", 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot_id"})
		return
	}

	filter := domain.ListFilter{
		Page:      page,
		PageSize:  pageSize,
		UserID:    c.Query("user_id"),
		ParkingID: parkingID,
		SpotID:    spotID,
		Status:    domain.BookingStatus(strings.TrimSpace(c.Query("status"))),
	}

	bookings, total, err := h.service.ListBookings(c.Request.Context(), filter)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     bookings,
		"page":      filter.Page,
		"page_size": filter.PageSize,
		"total":     total,
	})
}

func (h *Handler) ConfirmBooking(c *gin.Context) {
	booking, err := h.service.ConfirmBooking(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *Handler) CancelBooking(c *gin.Context) {
	var req cancelBookingRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	booking, err := h.service.CancelBooking(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *Handler) StartBooking(c *gin.Context) {
	booking, err := h.service.StartBooking(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *Handler) CompleteBooking(c *gin.Context) {
	booking, err := h.service.CompleteBooking(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrBookingConflict), errors.Is(err, domain.ErrInvalidStatusTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func parseIntQuery(c *gin.Context, key string, fallback int) (int, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func parseInt64Query(c *gin.Context, key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}
