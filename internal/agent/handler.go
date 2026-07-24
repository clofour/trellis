package agent

import (
	"net/http"

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

	err = h.agent.RunAllocation(ctx, request.JobName, request.GroupName, request.Name, request.Task)
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
