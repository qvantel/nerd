package api

import (
	"net/http"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/series"
)

// AddPoint godoc
// @Summary Metric ingestion endpoint
// @Description Will return the output produced by the given net for the given input
// @Accept json
// @Produce json
// @Success 202
// @Failure 400 {object} types.APIError "When the request body is formatted incorrectly"
// @Failure 500 {object} types.APIError "When there is an error processing the update"
// @Router /series/process [post]
func (h *Handler) AddPoint(c *gin.Context) {
	event := cloudevents.NewEvent()

	err := c.ShouldBind(&event)
	if err != nil {
		logger.Debug("Failed to unmarshal message (" + err.Error() + ")")
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "Wrong format"})
		return
	}

	err = series.ProcessUpdate(event, h.PS, h.TServ, h.Conf)
	if err != nil {
		logger.Error("Failed to process message", err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error processing point, see logs for more info"})
		return
	}

	c.String(http.StatusAccepted, "")
}

// DeleteSeries godoc
// @Summary Series deletion endpoint
// @Description Will delete the series with the specified ID
// @Produce json
// @Param id path string true "Series ID"
// @Success 200
// @Failure 404 {object} types.APIError "When the series doesn't exist"
// @Failure 500 {object} types.APIError "When there is an error deleting the series"
// @Router /series/{id} [delete]
func (h *Handler) DeleteSeries(c *gin.Context) {
	id := c.Param("id")
	delErr := types.APIError{Msg: "Error deleting series, see logs for more info"}
	found, err := h.PS.Exists(id)
	if err != nil {
		logger.Error("Failed to check if the series "+id+" exists in the store", err)
		c.JSON(http.StatusInternalServerError, delErr)
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, types.APIError{Msg: "Series with id " + id + " could not be found"})
		return
	}
	err = h.PS.DeleteSeries(id)
	if err != nil {
		logger.Error("Failed to delete series "+id, err)
		c.JSON(http.StatusInternalServerError, delErr)
		return
	}
	c.JSON(http.StatusOK, "")
}

// ListPoints godoc
// @Summary Retrieve points from series
// @Description Will return the last N points for the given series
// @Produce json
// @Param id path string true "Series ID"
// @Param limit query int false "How many points to fetch" default(10) maximum(500)
// @Success 200 {array} pointstores.Point
// @Failure 404 {object} types.APIError "When the series doesn't exist"
// @Failure 500 {object} types.APIError "When there is an error fetching the points"
// @Router /series/{id}/points [get]
func (h *Handler) ListPoints(c *gin.Context) {
	raw := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "limit must be a valid integer"})
	}
	if limit > 500 {
		limit = 500 // So things won't get too much out of control
	}
	id := c.Param("id")
	exists, err := h.PS.Exists(id)
	if err != nil {
		logger.Error("Failed to check if series with id "+id+" exists", err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error fetching points, see logs for more info"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, types.APIError{Msg: "Series with id " + id + " could not be found"})
		return
	}

	points, err := h.PS.GetLastN(id, nil, limit)
	if err != nil {
		logger.Error("Failed to get points from series with id " + id + " (" + err.Error() + ")")
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error fetching points, see logs for more info"})
		return
	}
	c.JSON(http.StatusOK, points)
}

// ListSeries godoc
// @Summary Retrieve list of series
// @Description Will return the list of series in the system
// @Produce json
// @Success 200 {array} types.BriefSeries
// @Failure 500 {object} types.APIError "When there is an error fetching the list of series"
// @Router /series [get]
func (h *Handler) ListSeries(c *gin.Context) {
	series, err := h.PS.ListSeries()
	if err != nil {
		logger.Error("Failed to get list of series", err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error getting list of series, see logs for more info"})
		return
	}
	c.JSON(http.StatusOK, series)
}
