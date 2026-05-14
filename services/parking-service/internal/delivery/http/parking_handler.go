package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
)

type ParkingHandler struct {
	parkingUC *usecase.ParkingUsecase
}

func NewParkingHandler(parkingUC *usecase.ParkingUsecase) *ParkingHandler {
	return &ParkingHandler{
		parkingUC: parkingUC,
	}
}

type CreateParkingRequest struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	TotalSpots int    `json:"total_spots"`
}

func (h *ParkingHandler) CreateParking(c *gin.Context) {
	var req CreateParkingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parking, err := h.parkingUC.CreateParking(req.Name, req.Address, req.TotalSpots)
	if err != nil {
		writeParkingError(c, err)
		return
	}

	c.JSON(http.StatusCreated, parking)
}

func (h *ParkingHandler) GetParking(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parking id"})
		return
	}

	parking, err := h.parkingUC.GetParking(id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "parking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, parking)
}

func (h *ParkingHandler) GetAllParkings(c *gin.Context) {
	parkings, err := h.parkingUC.GetAllParkings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, parkings)
}

func writeParkingError(c *gin.Context, err error) {
	if err == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unknown error"})
		return
	}

	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "required"), strings.Contains(msg, "must be greater than zero"), strings.Contains(msg, "invalid"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case strings.Contains(msg, "limit reached"):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
