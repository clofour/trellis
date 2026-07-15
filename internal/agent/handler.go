package agent

import (
	"net/http"

	"github.com/clofour/trellis/internal/spec"
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
	Name        string
	Image       string
	Env         map[string]string
	Ports       []PortRequest
	Volumes     []VolumeRequest
	HealthCheck HealthCheckRequest
}

type PortRequest struct {
	HostPort      int
	ContainerPort int
}

type VolumeRequest struct {
	Name string
	Path string
}

type HealthCheckRequest struct {
	Type    string
	Port    int
	Path    string
	Command []string
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

func convertAlloc(req AllocationRequest) *spec.TaskSpec {
	volumes := make([]spec.VolumeSpec, 0, len(req.Volumes))
	for _, volume := range req.Volumes {
		volumes = append(volumes, spec.VolumeSpec{
			Name: volume.Name,
			Path: volume.Path,
		})
	}

	ports := make([]spec.PortSpec, 0, len(req.Volumes))
	for _, port := range req.Ports {
		ports = append(ports, spec.PortSpec{
			HostPort:      port.HostPort,
			ContainerPort: port.ContainerPort,
		})
	}

	healthCheckRequest := req.HealthCheck
	healthCheck := &spec.HealthCheckSpec{
		Type:    healthCheckRequest.Type,
		Port:    healthCheckRequest.Port,
		Path:    healthCheckRequest.Path,
		Command: healthCheckRequest.Command,
	}

	return &spec.TaskSpec{
		Name:        req.Name,
		Image:       req.Image,
		Env:         req.Env,
		HealthCheck: healthCheck,
		Ports:       ports,
		Volumes:     volumes,
	}
}
