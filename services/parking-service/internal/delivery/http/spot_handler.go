package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"parking-service/internal/domain"
	"parking-service/internal/usecase"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "spot not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "spot deleted"})
}
