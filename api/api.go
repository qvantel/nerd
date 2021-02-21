// Package api contains the handlers and types that support nerd's REST API
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/nets/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

const base = "/api"

// Handler holds the API's state
type Handler struct {
	Conf   config.Config
	NPS    paramstores.NetParamStore
	PS     pointstores.PointStore
	Router *gin.Engine
	TServ  chan types.TrainRequest
}

// @title Nerd
// @version 1.0
// @description Nerd provides machine learning as a service.

// @host localhost:5400
// @BasePath /api/v1

// New initializes the Gin rest api and returns a handler
func New(tServ chan types.TrainRequest, conf config.Config) (*Handler, error) {
	// Set up net param store
	nps, err := paramstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize net param store for the API", err)
		return nil, err
	}

	// Set up point store
	ps, err := pointstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize point store for the API", err)
		return nil, err
	}

	router := gin.New()

	h := Handler{
		Conf:   conf,
		NPS:    nps,
		PS:     ps,
		Router: router,
		TServ:  tServ,
	}

	// Global middleware
	router.Use(gin.LoggerWithFormatter(logger.GinFormatter))
	router.Use(gin.Recovery())

	// Routes
	router.GET("/", h.ShowWelcomeMsg)
	v1 := router.Group(base + "/v1")
	{
		health := v1.Group("/health")
		{
			health.GET("/startup", StartupCheck)
		}
		nets := v1.Group("/nets")
		{
			nets.GET("", h.ListNets)
			nets.POST("", h.Train)
			nets.DELETE("/:id", h.DeleteNet)
			nets.POST("/:id/evaluate", h.Evaluate)
		}
		series := v1.Group("/series")
		{
			series.GET("", h.ListSeries)
			series.DELETE("/:id", h.DeleteSeries)
			series.GET("/:id/nets", h.ListSeriesNets)
			series.GET("/:id/points", h.ListPoints)
			series.POST("/process", h.ProcessEvent)
		}
	}

	logger.Info("API initialized")
	return &h, nil
}

// ShowWelcomeMsg is a simple handler method for showing a helpful message at the root of the HTTP server
func (h *Handler) ShowWelcomeMsg(c *gin.Context) {
	msg := `<!doctype html>
<html>
	<body>
		<h1>nerd ` + h.Conf.AppVersion + `</h1>

		Welcome to the nerd API! This is a restful machine learning service, if you'd like to learn more about it, maybe
		check out the github project <a href="https://github.com/qvantel/nerd">here</a>.
	</body>
</html>`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write([]byte(msg))
}
