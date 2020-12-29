package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/api/types"
)

// StartupCheck godoc
// @Summary Kubernetes startup probe endpoint
// @Description Will return a 200 as long as the API is up
// @Produce plain
// @Success 200 {object} types.SimpleRes
// @Router /health/startup [get]
func StartupCheck(c *gin.Context) {
	c.JSON(http.StatusOK, types.NewOkRes("The API is up"))
}
