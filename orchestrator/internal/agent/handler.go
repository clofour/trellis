package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/clofour/trellis/internal/api"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	agent *Agent
}

func NewHandler(agent *Agent) *Handler {
	return &Handler{
		agent: agent,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	v1 := e.Group("/v1")
	v1.GET("/allocations", h.handleList)
	v1.POST("/allocations", h.handleRun)
	v1.DELETE("/allocations/:id", h.handleDelete)
	v1.GET("/allocations/:id/logs", h.handleLogs)
}

func (h *Handler) handleLogs(c *echo.Context) error {
	tail, err := strconv.Atoi(c.QueryParam("tail"))
	if c.QueryParam("tail") == "" {
		tail, err = 100, nil
	}
	if err != nil || tail < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "tail must be a non-negative integer")
	}
	logs, err := h.agent.Logs(c.Request().Context(), c.Param("id"), c.QueryParam("follow") == "true", tail)
	if err != nil {
		if errors.Is(err, ErrAllocationNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "allocation not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to read allocation logs")
	}
	defer logs.Close()
	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), logs)
	return err
}

func (h *Handler) handleList(c *echo.Context) error {
	allocs := h.agent.GetAllocations()
	return c.JSON(http.StatusOK, allocs)
}

func (h *Handler) handleRun(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.AllocationRequest
	err := c.Bind(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if len(request.Tasks) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "allocation tasks are required")
	}
	if request.AllocationID == "" {
		request.AllocationID = request.Name
	}
	if request.Generation == 0 {
		request.Generation = 1
	}
	if request.ExecutionHash == "" {
		copy := request
		copy.Epoch, copy.ExecutionHash = 0, ""
		raw, _ := json.Marshal(copy)
		sum := sha256.Sum256(raw)
		request.ExecutionHash = hex.EncodeToString(sum[:])
	}
	if err := h.agent.PrepareStart(ctx, &request); err != nil {
		return operationError(err)
	}
	for i := range request.Tasks {
		task := &request.Tasks[i]
		id := fmt.Sprintf("%s-g%d-%s", request.AllocationID, request.Generation, task.Name)
		err = h.agent.RunAllocation(ctx, id, request.AllocationID, request.Generation, request.JobRevision, request.ExecutionHash, request.Namespace, request.JobName, request.GroupName, task.Name, task, request.Runtime, request.WireGuard, request.NetworkPlan, request.NetworkMode, request.EnvOverrides, request.Restart)
		if err != nil {
			break
		}
	}
	if err != nil {
		h.agent.log.Error("start allocation failed", "allocation", request.Name, "error", err)
		return operationError(err)
	}

	return c.JSON(http.StatusOK, api.OperationResponse{Code: api.OperationOK, Generation: request.Generation, Epoch: request.Epoch})
}

func (h *Handler) handleDelete(c *echo.Context) error {
	ctx := c.Request().Context()

	request := api.StopAllocationRequest{AllocationID: c.Param("id")}
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if request.AllocationID == "" {
		request.AllocationID = c.Param("id")
	}
	err := h.agent.StopGroup(ctx, &request)
	if err != nil {
		return operationError(err)
	}

	return c.JSON(http.StatusOK, api.OperationResponse{Code: api.OperationOK, Generation: request.Generation, Epoch: request.Epoch})
}

func operationError(err error) error {
	status, code := http.StatusInternalServerError, api.OperationFailed
	switch {
	case errors.Is(err, ErrStaleEpoch):
		status, code = http.StatusConflict, api.OperationStaleEpoch
	case errors.Is(err, ErrStaleGeneration):
		status, code = http.StatusConflict, api.OperationStaleGeneration
	case errors.Is(err, ErrExecutionConflict), errors.Is(err, ErrAllocationExists):
		status, code = http.StatusConflict, api.OperationConflict
	}
	return echo.NewHTTPError(status, api.OperationResponse{Code: code, Message: err.Error()})
}
