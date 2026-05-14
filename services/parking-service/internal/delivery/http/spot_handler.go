package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

type SpotHandler struct {
	spotUC *usecase.SpotUsecase
}

func NewSpotHandler(spotUC *usecase.SpotUsecase) *SpotHandler {
	return &SpotHandler{spotUC: spotUC}
}

type CreateSpotRequest struct {
	ParkingID int64  `json:"parking_id"`
	Number    string `json:"number"`
}

type UpdateSpotStatusRequest struct {
	Status string `json:"status"`
}

func (h *SpotHandler) CreateSpot(c *gin.Context) {
	var req CreateSpotRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	spot, err := h.spotUC.CreateSpot(req.ParkingID, req.Number)
	if err != nil {
		writeSpotError(c, err)
		return
	}

	c.JSON(http.StatusCreated, spot)
}

func (h *SpotHandler) GetSpot(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot id"})
		return
	}

	spot, err := h.spotUC.GetSpot(id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "spot not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, spot)
}

func (h *SpotHandler) GetSpotsByParking(c *gin.Context) {
	parkingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking id"})
		return
	}

	spots, err := h.spotUC.GetSpotsByParking(parkingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, spots)
}

func (h *SpotHandler) UpdateSpotStatus(c *gin.Context) {
	spotID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot id"})
		return
	}

	var req UpdateSpotStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.spotUC.UpdateSpotStatus(spotID, domain.SpotStatus(req.Status))
	if err != nil {
		writeSpotError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "spot status updated"})
}

func (h *SpotHandler) ReserveSpot(c *gin.Context) {
	spotID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot id"})
		return
	}

	if err := h.spotUC.ReserveSpot(spotID); err != nil {
		writeSpotError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "spot reserved"})
}

func (h *SpotHandler) ReleaseSpot(c *gin.Context) {
	spotID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot id"})
		return
	}

	if err := h.spotUC.ReleaseSpot(spotID); err != nil {
		writeSpotError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "spot released"})
}

func (h *SpotHandler) DeleteSpot(c *gin.Context) {
	spotID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spot id"})
		return
	}

	if err := h.spotUC.DeleteSpot(spotID); err != nil {
		writeSpotError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "spot deleted"})
}

func writeSpotError(c *gin.Context, err error) {
	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "required"), strings.Contains(msg, "invalid"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case strings.Contains(msg, "not available"), strings.Contains(msg, "not reserved"), strings.Contains(msg, "already"):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case strings.Contains(msg, "limit reached"):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
