package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/service"
)

type PaymentHandler struct {
	service *service.PaymentService
}

type createPaymentByBookingRequest struct {
	Method string `json:"method"`
}

func NewPaymentHandler(service *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.Health)
	router.GET("/ready", h.Health)

	payments := router.Group("/payments")
	{
		payments.POST("", h.CreatePayment)
		payments.POST("/booking/:booking_id", h.CreatePaymentByBooking)
		payments.GET("", h.ListPayments)
		payments.GET("/:id", h.GetPayment)
		payments.GET("/booking/:booking_id", h.GetPaymentByBooking)
		payments.POST("/:id/cancel", h.CancelPayment)
		payments.GET("/health", h.Health)
		payments.GET("/ready", h.Health)
	}
}

func (h *PaymentHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "payment-service",
	})
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var input domain.CreatePaymentInput
	if err := decodeJSON(c, &input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), input)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"payment": payment,
	})
}

func (h *PaymentHandler) CreatePaymentByBooking(c *gin.Context) {
	bookingID := strings.TrimSpace(c.Param("booking_id"))
	if bookingID == "" {
		writeError(c, http.StatusBadRequest, "booking id is required")
		return
	}

	method := domain.MethodCard
	if c.Request.ContentLength > 0 {
		var req createPaymentByBookingRequest
		if err := decodeJSON(c, &req); err != nil {
			writeError(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(req.Method) != "" {
			method = domain.PaymentMethod(strings.TrimSpace(req.Method))
		}
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), domain.CreatePaymentInput{
		BookingID: bookingID,
		Method:    method,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"payment": payment,
	})
}

func (h *PaymentHandler) GetPayment(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeError(c, http.StatusBadRequest, "payment id is required")
		return
	}

	payment, err := h.service.GetPayment(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"payment": payment,
	})
}

func (h *PaymentHandler) GetPaymentByBooking(c *gin.Context) {
	bookingID := strings.TrimSpace(c.Param("booking_id"))
	if bookingID == "" {
		writeError(c, http.StatusBadRequest, "booking id is required")
		return
	}

	payment, err := h.service.GetPaymentByBooking(c.Request.Context(), bookingID)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"payment": payment,
	})
}

func (h *PaymentHandler) ListPayments(c *gin.Context) {
	filter := domain.ListPaymentsFilter{
		UserID: strings.TrimSpace(c.Query("user_id")),
		Status: strings.TrimSpace(c.Query("status")),
	}

	payments, err := h.service.ListPayments(c.Request.Context(), filter)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"payments": payments,
		"count":    len(payments),
	})
}

func (h *PaymentHandler) CancelPayment(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeError(c, http.StatusBadRequest, "payment id is required")
		return
	}

	payment, err := h.service.CancelPayment(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"payment": payment,
	})
}

func decodeJSON(c *gin.Context, dst any) error {
	defer c.Request.Body.Close()

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if dec.More() {
		return errors.New("unexpected extra data")
	}

	return nil
}

func writeError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"error":   message,
	})
}

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrPaymentNotFound):
		writeError(c, http.StatusNotFound, "payment not found")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrPaymentAlreadyPaid):
		writeError(c, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrBookingAlreadyPaid):
		writeError(c, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidMethod):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "already in progress"):
			writeError(c, http.StatusConflict, err.Error())
		case strings.Contains(msg, "already paid"):
			writeError(c, http.StatusConflict, err.Error())
		case strings.Contains(msg, "cannot be paid"):
			writeError(c, http.StatusUnprocessableEntity, err.Error())
		case strings.Contains(msg, "not found"):
			writeError(c, http.StatusNotFound, err.Error())
		case strings.Contains(msg, "required"):
			writeError(c, http.StatusBadRequest, err.Error())
		case strings.Contains(msg, "invalid"):
			writeError(c, http.StatusBadRequest, err.Error())
		default:
			writeError(c, http.StatusInternalServerError, err.Error())
		}
	}
}
