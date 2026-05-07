package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"payment-service/internal/domain"
	"payment-service/internal/repository"
	"payment-service/internal/service"
)

type PaymentHandler struct {
	service *service.PaymentService
}

func NewPaymentHandler(service *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.Health)

	payments := router.Group("/payments")
	{
		payments.POST("", h.CreatePayment)
		payments.GET("", h.ListPayments)
		payments.GET("/:id", h.GetPayment)
		payments.GET("/booking/:booking_id", h.GetPaymentByBooking)
		payments.POST("/:id/cancel", h.CancelPayment)
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

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, payment)
}

func (h *PaymentHandler) GetPayment(c *gin.Context) {
	id := c.Param("id")

	payment, err := h.service.GetPayment(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "payment not found",
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}

func (h *PaymentHandler) GetPaymentByBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")

	payment, err := h.service.GetPaymentByBooking(c.Request.Context(), bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "payment not found",
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}

func (h *PaymentHandler) ListPayments(c *gin.Context) {
	filter := domain.ListPaymentsFilter{
		UserID: c.Query("user_id"),
		Status: c.Query("status"),
	}

	payments, err := h.service.ListPayments(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payments": payments,
	})
}

func (h *PaymentHandler) CancelPayment(c *gin.Context) {
	id := c.Param("id")

	payment, err := h.service.CancelPayment(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "payment not found",
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}
