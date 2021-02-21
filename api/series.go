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

// DeleteSeries godoc
// @Summary Series deletion endpoint
// @Description Will delete the series with the specified ID
// @Produce json
// @Param id path string true "Series ID"
// @Success 200 {object} types.SimpleRes
// @Failure 404 {object} types.SimpleRes "When the series doesn't exist"
// @Failure 500 {object} types.SimpleRes "When there is an error deleting the series"
// @Router /series/{id} [delete]
func (h *Handler) DeleteSeries(c *gin.Context) {
	id := c.Param("id")
	delErr := types.NewErrorRes("Error deleting series, see logs for more info")
	found, err := h.PS.Exists(id)
	if err != nil {
		logger.Error("Failed to check if the series "+id+" exists in the store", err)
		c.JSON(http.StatusInternalServerError, delErr)
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, types.NewErrorRes("Series with ID "+id+" could not be found"))
		return
	}
	err = h.PS.DeleteSeries(id)
	if err != nil {
		logger.Error("Failed to delete series "+id, err)
		c.JSON(http.StatusInternalServerError, delErr)
		return
	}
	c.JSON(http.StatusOK, types.NewOkRes("Series "+id+" was successfully deleted"))
}

// ListPoints godoc
// @Summary Retrieve points from series
// @Description Will return the last N points for the given series
// @Produce json
// @Param id path string true "Series ID"
// @Param limit query int false "How many points to fetch" default(10) maximum(500)
// @Success 200 {array} pointstores.Point
// @Failure 404 {object} types.SimpleRes "When the series doesn't exist"
// @Failure 500 {object} types.SimpleRes "When there is an error fetching the points"
// @Router /series/{id}/points [get]
func (h *Handler) ListPoints(c *gin.Context) {
	raw := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorRes("limit must be a valid integer"))
	}
	if limit > 500 {
		limit = 500 // So things won't get too much out of control
	}
	id := c.Param("id")
	exists, err := h.PS.Exists(id)
	if err != nil {
		logger.Error("Failed to check if series with ID "+id+" exists", err)
		c.JSON(http.StatusInternalServerError, types.NewErrorRes("Error fetching points, see logs for more info"))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, types.NewErrorRes("Series with ID "+id+" could not be found"))
		return
	}

	points, err := h.PS.GetLastN(id, nil, limit)
	if err != nil {
		logger.Error("Failed to get points from series with ID " + id + " (" + err.Error() + ")")
		c.JSON(http.StatusInternalServerError, types.NewErrorRes("Error fetching points, see logs for more info"))
		return
	}
	c.JSON(http.StatusOK, points)
}

// ListSeries godoc
// @Summary Retrieve list of series
// @Description Will return the list of series in the system
// @Produce json
// @Success 200 {array} types.BriefSeries
// @Failure 500 {object} types.SimpleRes "When there is an error fetching the list of series"
// @Router /series [get]
func (h *Handler) ListSeries(c *gin.Context) {
	series, err := h.PS.ListSeries()
	if err != nil {
		logger.Error("Failed to get list of series", err)
		c.JSON(http.StatusInternalServerError, types.NewErrorRes("Error getting list of series, see logs for more info"))
		return
	}
	c.JSON(http.StatusOK, series)
}

// ListSeriesNets godoc
// @Summary Alias for retrieving the nets for the given series
// @Description Will return the list of nets in the system trained with the given series
// @Produce json
// @Param id path string true "Series ID"
// @Param offset query int false "Offset to fetch" default(0)
// @Param limit query int false "How many networks to fetch, the service might return more in some cases" default(10) maximum(50)
// @Success 200 {object} types.PagedRes
// @Failure 400 {object} types.SimpleRes "When the request params are formatted incorrectly"
// @Failure 500 {object} types.SimpleRes "When there is an error fetching the list of nets"
// @Router /series/{id}/nets [get]
func (h *Handler) ListSeriesNets(c *gin.Context) {
	id := c.Param("id")
	exists, err := h.PS.Exists(id)
	if err != nil {
		logger.Error("Failed to check if series with ID "+id+" exists", err)
		c.JSON(http.StatusInternalServerError, types.NewErrorRes("Error fetching nets, see logs for more info"))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, types.NewErrorRes("Series with ID "+id+" could not be found"))
		return
	}

	c.Set("seriesID", id)
	h.ListNets(c)
}

// ProcessEvent godoc
// @Summary Metric ingestion endpoint
// @Description Will process the provided metrics update
// @Accept json
// @Produce json
// @Success 202 {object} types.SimpleRes
// @Failure 400 {object} types.SimpleRes "When the request body is formatted incorrectly"
// @Failure 500 {object} types.SimpleRes "When there is an error processing the update"
// @Router /series/process [post]
func (h *Handler) ProcessEvent(c *gin.Context) {
	event := cloudevents.NewEvent()

	err := c.ShouldBind(&event)
	if err != nil {
		logger.Debug("Failed to unmarshal event (" + err.Error() + ")")
		c.JSON(http.StatusBadRequest, types.NewErrorRes("Wrong format"))
		return
	}

	err = series.ProcessUpdate(event, h.PS, h.TServ, h.Conf)
	if err != nil {
		logger.Error("Failed to process event", err)
		c.JSON(http.StatusInternalServerError, types.NewErrorRes("Error processing event, see logs for more info"))
		return
	}

	c.JSON(http.StatusAccepted, types.NewOkRes("Metrics update processed successfully"))
}
