package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StartupCheck godoc
// @Summary Kubernetes startup probe endpoint
// @Description Will return a 200 as long as the API is up
// @Produce plain
// @Success 200 {string} string
// @Router /health/startup [get]
func StartupCheck(c *gin.Context) {
	c.String(http.StatusOK, "UP")
}
