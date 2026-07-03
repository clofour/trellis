package agent

import (
	"net/http"

	"github.com/clofour/trellis/internal/models"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	agent *Agent
}

type RunRequest struct {
	JobName     string
	GroupName   string
	Allocations []AllocationRequest
}

type AllocationRequest struct {
	Name    string
	Image   string
	Env     map[string]string
	Ports   []string
	Volumes []VolumeRequest
}

type VolumeRequest struct {
	Name string
	Path string
}

func NewHandler(agent *Agent) *Handler {
	return &Handler{
		agent: agent,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/allocations", h.handleList)
	e.POST("/allocations", h.handleRun)
	e.DELETE("/allocations/:id", h.handleDelete)
}

func (h *Handler) handleList(c *echo.Context) error {
	ctx := c.Request().Context()

	allocs := h.agent.GetAllocations(ctx)
	return c.JSON(http.StatusOK, allocs)
}

func (h *Handler) handleRun(c *echo.Context) error {
	ctx := c.Request().Context()

	var req RunRequest
	err := c.Bind(&req)
	if err != nil {
		return err
	}

	for _, rawAlloc := range req.Allocations {

		alloc := convertAlloc(rawAlloc)
		err := h.agent.RunAllocation(ctx, req.JobName, req.GroupName, alloc.Name, alloc)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

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

func convertAlloc(req AllocationRequest) *models.TaskSpec {
	volumes := make([]models.VolumeSpec, 0, len(req.Volumes))
	for _, volume := range req.Volumes {
		volumes = append(volumes, models.VolumeSpec{
			Name: volume.Name,
			Path: volume.Path,
		})
	}

	return &models.TaskSpec{
		Name:    req.Name,
		Image:   req.Image,
		Ports:   req.Ports,
		Env:     req.Env,
		Volumes: volumes,
	}
}
