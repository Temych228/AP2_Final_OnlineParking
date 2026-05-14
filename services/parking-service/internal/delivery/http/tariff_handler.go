package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

type TariffHandler struct {
	tariffUC *usecase.TariffUsecase
}

func NewTariffHandler(tariffUC *usecase.TariffUsecase) *TariffHandler {
	return &TariffHandler{tariffUC: tariffUC}
}

type CreateTariffRequest struct {
	ParkingID    int64   `json:"parking_id"`
	PricePerHour float64 `json:"price_per_hour"`
}

type UpdateTariffRequest struct {
	PricePerHour float64 `json:"price_per_hour"`
}

func (h *TariffHandler) CreateTariff(c *gin.Context) {
	var req CreateTariffRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tariff, err := h.tariffUC.CreateTariff(req.ParkingID, req.PricePerHour)
	if err != nil {
		writeTariffError(c, err)
		return
	}

	c.JSON(http.StatusCreated, tariff)
}

func (h *TariffHandler) GetTariff(c *gin.Context) {
	parkingID, err := strconv.ParseInt(c.Param("parking_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking id"})
		return
	}

	tariff, err := h.tariffUC.GetTariff(parkingID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "tariff not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tariff)
}

func (h *TariffHandler) UpdateTariff(c *gin.Context) {
	parkingID, err := strconv.ParseInt(c.Param("parking_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking id"})
		return
	}

	var req UpdateTariffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tariffUC.UpdateTariff(parkingID, req.PricePerHour); err != nil {
		writeTariffError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tariff updated"})
}

func (h *TariffHandler) CalculatePrice(c *gin.Context) {
	parkingID, err := strconv.ParseInt(c.Param("parking_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking id"})
		return
	}

	hours, err := strconv.ParseFloat(c.Query("hours"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hours"})
		return
	}

	price, err := h.tariffUC.CalculatePrice(parkingID, hours)
	if err != nil {
		writeTariffError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"total_price": price})
}

func writeTariffError(c *gin.Context, err error) {
	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "required"), strings.Contains(msg, "invalid"), strings.Contains(msg, "must be greater than zero"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
