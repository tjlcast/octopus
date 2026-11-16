package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/server/middleware"
	"github.com/bestruirui/octopus/internal/server/resp"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
)

func init() {
	router.NewGroupRouter("/api/v1/log").
		Use(middleware.Auth()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listLog),
		).
		AddRoute(
			router.NewRoute("/clear", http.MethodDelete).
				Handle(clearLog),
		).
		AddRoute(
			router.NewRoute("/stream-token", http.MethodGet).
				Handle(getStreamToken),
		)

	router.NewGroupRouter("/api/v1/log").
		AddRoute(
			router.NewRoute("/stream", http.MethodGet).
				Handle(streamLog),
		)
}

func listLog(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filters, err := parseLogFilters(c)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	logs, err := op.RelayLogList(c.Request.Context(), filters, page, pageSize)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp.Success(c, logs)
}

func parseLogFilters(c *gin.Context) (op.RelayLogFilter, error) {
	filter := op.RelayLogFilter{
		RequestModelName: c.Query("request_model_name"),
		ActualModelName:  c.Query("actual_model_name"),
	}

	var err error
	for _, item := range []struct {
		name   string
		target **int
	}{
		{"start_time", &filter.StartTime},
		{"end_time", &filter.EndTime},
		{"min_input_tokens", &filter.MinInputTokens},
		{"max_input_tokens", &filter.MaxInputTokens},
		{"min_output_tokens", &filter.MinOutputTokens},
		{"max_output_tokens", &filter.MaxOutputTokens},
		{"min_ftut", &filter.MinFtut},
		{"max_ftut", &filter.MaxFtut},
		{"min_use_time", &filter.MinUseTime},
		{"max_use_time", &filter.MaxUseTime},
		{"channel_id", &filter.ChannelID},
	} {
		if *item.target, err = optionalNonNegativeInt(c.Query(item.name), item.name); err != nil {
			return op.RelayLogFilter{}, err
		}
	}

	filter.Status = c.Query("status")
	if filter.Status != "" && filter.Status != "success" && filter.Status != "failed" {
		return op.RelayLogFilter{}, fmt.Errorf("status must be success or failed")
	}
	if err := filter.Validate(); err != nil {
		return op.RelayLogFilter{}, err
	}
	return filter, nil
}

func optionalNonNegativeInt(value, name string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return &parsed, nil
}

func clearLog(c *gin.Context) {
	if err := op.RelayLogClear(c.Request.Context()); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, nil)
}

func getStreamToken(c *gin.Context) {
	token, err := op.RelayLogStreamTokenCreate()
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, gin.H{"token": token})
}

func streamLog(c *gin.Context) {
	token := c.Query("token")
	if token == "" || !op.RelayLogStreamTokenVerify(token) {
		resp.Error(c, http.StatusUnauthorized, "invalid stream token")
		return
	}

	op.RelayLogStreamTokenRevoke(token)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	logChan := op.RelayLogSubscribe()
	defer op.RelayLogUnsubscribe(logChan)

	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case log, ok := <-logChan:
			if !ok {
				return
			}
			data, err := json.Marshal(log)
			if err != nil {
				continue
			}
			c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			c.Writer.Flush()
		}
	}
}
