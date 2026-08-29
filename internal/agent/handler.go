package agent

import (
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
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	defer logs.Close()
	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), logs)
	return err
}

func (h *Handler) handleList(c *echo.Context) error {
	ctx := c.Request().Context()

	allocs := h.agent.GetAllocations(ctx)
	return c.JSON(http.StatusOK, allocs)
}

func (h *Handler) handleRun(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.AllocationRequest
	err := c.Bind(&request)
	if err != nil {
		return err
	}

	err = h.agent.RunAllocation(ctx, request.Name, request.Tenant, request.JobName, request.GroupName, request.Name, request.Task, request.Isolation)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) handleDelete(c *echo.Context) error {
	ctx := c.Request().Context()

	id := c.Param("id")

	err := h.agent.StopAllocation(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusOK)
}
