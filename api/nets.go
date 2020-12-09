package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml"
)

// DeleteNet godoc
// @Summary Network deletion endpoint
// @Description Will delete the net with the specified ID
// @Produce json
// @Param id path string true "Net ID"
// @Success 200
// @Failure 500 {object} types.APIError "When there is an error deleting the net"
// @Router /nets/{id} [delete]
func (h *Handler) DeleteNet(c *gin.Context) {
	id := c.Param("id")
	err := h.MLS.Delete(id)
	if err != nil {
		logger.Error("Failed to delete net "+id, err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error deleting net, see logs for more info"})
		return
	}
	c.JSON(http.StatusOK, "")
}

// Evaluate godoc
// @Summary Input evaluation endpoint
// @Description Will return the output produced by the given net for the given input
// @Accept json
// @Produce json
// @Param id path string true "Net ID"
// @Success 200 {object} map[string]float32
// @Failure 400 {object} types.APIError "When the request body is formatted incorrectly"
// @Failure 404 {object} types.APIError "When the provided net ID isn't found"
// @Failure 500 {object} types.APIError "When there is an error loading the net or evaluating the inputs"
// @Router /nets/{id}/evaluate [post]
func (h *Handler) Evaluate(c *gin.Context) {
	id := c.Param("id")
	var inputs map[string]float32
	err := c.ShouldBind(&inputs)
	if err != nil {
		logger.Debug("Failed to unmarshal message (" + err.Error() + ")")
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "Wrong format"})
		return
	}

	_, err = ml.ID2Type(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIError{Msg: err.Error()})
		return
	}

	// The net has to exist for this to work so it's more memory efficient to pass the zero value of each type for the
	// creation params
	net, err := ml.NewNetwork(id, nil, nil, 0, false, h.MLS, h.Conf)
	loadErr := types.APIError{Msg: "Error loading net, see logs for more info"}
	if err != nil {
		logger.Error("Failed to load net "+id, err)
		c.JSON(http.StatusInternalServerError, loadErr)
		return
	}
	if net == nil {
		c.JSON(http.StatusNotFound, types.APIError{Msg: "Net with id " + id + " could not be found"})
		return
	}
	res, err := net.Evaluate(inputs)
	if err != nil {
		logger.Error("Failed to evaluate inputs with net "+id, err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error evaluting inputs, see logs for more info"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListNets godoc
// @Summary Nets endpoint
// @Description Will return the paginated list of neural nets in the system
// @Produce json
// @Param offset query int false "Offset to fetch" default(0)
// @Param limit query int false "How many networks to fetch, the service might return more in some cases" default(10) maximum(50)
// @Success 200 {object} types.PagedRes
// @Failure 400 {object} types.APIError "When the request params are formatted incorrectly"
// @Failure 500 {object} types.APIError "When there is an error retrieving the list of nets"
// @Router /nets [get]
func (h *Handler) ListNets(c *gin.Context) {
	raw := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "offset must be a valid integer"})
	}
	raw = c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "limit must be a valid integer"})
	}
	if limit > 50 {
		limit = 50 // Till there is a better solution in place, this is so things won't get too much out of control
	}

	nets, cursor, err := ml.List(offset, limit, h.MLS)
	if err != nil {
		logger.Error("Failed to get list of nets", err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error getting list of nets, see logs for more info"})
		return
	}
	c.JSON(http.StatusOK, types.PagedRes{Last: cursor == 0, Next: cursor, Results: nets})
}

// Train godoc
// @Summary Net train endpoint
// @Description Used for training new or existing networks with the points from an existing series
// @Accept json
// @Success 200
// @Failure 400 {object} types.APIError "When the request body is formatted incorrectly"
// @Failure 404 {object} types.APIError "When the provided series ID isn't found"
// @Failure 500 {object} types.APIError "When there is an error processing the request"
// @Router /nets [post]
func (h *Handler) Train(c *gin.Context) {
	var tr types.TrainRequest
	err := c.ShouldBind(&tr)
	if err != nil {
		logger.Debug("Failed to unmarshal message (" + err.Error() + ")")
		c.JSON(http.StatusBadRequest, types.APIError{Msg: "Wrong format"})
		return
	}
	exists, err := h.PS.Exists(tr.SeriesID)
	if err != nil {
		logger.Error("Failed to check if series with id "+tr.SeriesID+" exists", err)
		c.JSON(http.StatusInternalServerError, types.APIError{Msg: "Error processing training request, see logs for more info"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, types.APIError{Msg: "Series with id " + tr.SeriesID + " could not be found"})
		return
	}
	sort.Strings(tr.Inputs)
	sort.Strings(tr.Outputs)
	h.TServ <- tr
	c.JSON(http.StatusAccepted, "")
}
