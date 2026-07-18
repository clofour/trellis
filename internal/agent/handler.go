package agent

import (
	"net/http"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
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
	v1 := e.Group("v1")
	v1.GET("/v1/allocations", h.handleList)
	v1.POST("/v1/allocations", h.handleRun)
	v1.DELETE("/v1/allocations/:id", h.handleDelete)
}

func (h *Handler) handleList(c *echo.Context) error {
	ctx := c.Request().Context()

	allocs := h.agent.GetAllocations(ctx)
	return c.JSON(http.StatusOK, allocs)
}

func (h *Handler) handleRun(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.RunRequest
	err := c.Bind(&request)
	if err != nil {
		return err
	}

	for _, rawAlloc := range request.Allocations {

		alloc := convertAlloc(rawAlloc)
		err := h.agent.RunAllocation(ctx, request.JobName, request.GroupName, alloc.Name, alloc)
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

func convertAlloc(request api.AllocationRequest) *spec.TaskSpec {
	volumes := make([]spec.VolumeSpec, 0, len(request.Volumes))
	for _, volume := range request.Volumes {
		volumes = append(volumes, spec.VolumeSpec{
			Name: volume.Name,
			Path: volume.Path,
		})
	}

	ports := make([]spec.PortSpec, 0, len(request.Volumes))
	for _, port := range request.Ports {
		ports = append(ports, spec.PortSpec{
			HostPort:      port.HostPort,
			ContainerPort: port.ContainerPort,
		})
	}

	healthCheckRequest := request.HealthCheck
	healthCheck := &spec.HealthCheckSpec{
		Type:    healthCheckRequest.Type,
		Port:    healthCheckRequest.Port,
		Path:    healthCheckRequest.Path,
		Command: healthCheckRequest.Command,
	}

	return &spec.TaskSpec{
		Name:        request.Name,
		Image:       request.Image,
		Env:         request.Env,
		HealthCheck: healthCheck,
		Ports:       ports,
		Volumes:     volumes,
	}
}
