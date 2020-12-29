// Package logger includes extended JSON logging functionality
package logger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qvantel/nerd/internal/config"
)

var (
	// LogLevel holds the application's current log level
	LogLevel int
	// ArtifactID holds the image being used to run the application
	ArtifactID string
	// ServiceName holds the application's name in the service discovery system
	ServiceName string
)

const (
	traceLevel   = 5000
	debugLevel   = 10000
	infoLevel    = 20000
	warningLevel = 30000
	errorLevel   = 40000
)

// EventLogData contains the needed information to be rendered as a logstash compatible json event log
type EventLogData struct {
	TimeStamp   string `json:"@timestamp"`
	Version     string `json:"@version"`
	LogType     string `json:"log_type"`
	LogLevel    string `json:"log_level"`
	LevelValue  int    `json:"level_value"`
	ServiceName string `json:"service_name"`
	LoggerName  string `json:"logger_name"`
	ArtifactID  string `json:"artifact_id"`
	TraceToken  string `json:"trace_token"`
	Message     string `json:"message"`
}

func newEvent(timeStamp, message, levelName string, levelValue int) *EventLogData {
	return &EventLogData{
		TimeStamp:   timeStamp,
		Version:     "1",
		LogType:     "LOG",
		LogLevel:    levelName,
		LevelValue:  levelValue,
		ServiceName: ServiceName,
		LoggerName:  ServiceName,
		ArtifactID:  ArtifactID,
		TraceToken:  "undefined",
		Message:     message,
	}
}

func writeLog(message, levelName string, levelValue int) {
	if LogLevel > levelValue {
		return
	}
	now := time.Now()
	data := newEvent(TimeFormated(&now), message, levelName, levelValue)
	b, _ := json.Marshal(data)
	fmt.Println(string(b))
}

// Trace logs a message to stdout with the TRACE log level
func Trace(message string) {
	writeLog(message, "TRACE", traceLevel)
}

// Debug logs a message to stdout with the DEBUG log level
func Debug(message string) {
	writeLog(message, "DEBUG", debugLevel)
}

// Info logs a message to stdout with the INFO log level
func Info(message string) {
	writeLog(message, "INFO", infoLevel)
}

// Warning logs a message to stdout with the WARN log level
func Warning(message string) {
	writeLog(message, "WARN", warningLevel)
}

// Error logs a message to stdout with the ERROR log level
func Error(message string, errs ...error) {
	if len(errs) == 0 {
		writeLog(message, "ERROR", errorLevel)
	} else {
		writeLog(message+" ("+errs[0].Error()+")", "ERROR", errorLevel)
	}
}

// Init initializes the logger object
func Init(conf config.Config) {
	switch conf.Logger.Level {
	case "TRACE":
		LogLevel = traceLevel
	case "DEBUG":
		LogLevel = debugLevel
	case "INFO":
		LogLevel = infoLevel
	case "WARN":
		LogLevel = warningLevel
	case "ERROR":
		LogLevel = errorLevel
	default:
		LogLevel = infoLevel
	}
	ArtifactID = conf.Logger.ArtifactID
	ServiceName = conf.Logger.ServiceName
}

// TimeFormated Formats the provided time according to 'yyyy-MM-dd'T'HH:mm:ssZ'
func TimeFormated(time *time.Time) string {
	return time.Format("2006-01-02T15:04:05.999-0700")
}

// GinFormatter is used to adapt Gin's logging to the Qvantel standard
func GinFormatter(param gin.LogFormatterParams) string {
	lValue := traceLevel
	lLevel := "TRACE"

	if param.StatusCode >= 500 && param.StatusCode <= 599 {
		lValue = warningLevel
		lLevel = "WARN"
	}

	if LogLevel > lValue {
		return ""
	}

	entry := newEvent(
		TimeFormated(&param.TimeStamp),
		fmt.Sprintf("[%s] %s %s %s - %d (in %s) %s",
			param.ClientIP,
			param.Request.Proto,
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ErrorMessage,
		),
		lLevel,
		lValue,
	)

	b, _ := json.Marshal(entry)

	return string(b) + "\n"
}
