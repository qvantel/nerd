package api

import (
	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

// Handler holds the API's state
type Handler struct {
	Conf   config.Config
	MLS    paramstores.NetParamStore
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
	// Set up ML store
	mls, err := paramstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize ML store for the API", err)
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
		MLS:    mls,
		PS:     ps,
		Router: router,
		TServ:  tServ,
	}

	// Global middleware
	router.Use(gin.LoggerWithFormatter(logger.GinFormatter))
	router.Use(gin.Recovery())

	// Routes
	v1 := router.Group("/api/v1")
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
			series.GET("/:id/points", h.ListPoints)
			series.POST("/process", h.AddPoint)
		}
	}

	logger.Info("API initialized")
	return &h, nil
}
