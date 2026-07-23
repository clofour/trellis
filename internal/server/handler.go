package server

import (
	"net/http"

	"github.com/clofour/trellis/internal/api"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	server *Server
}

func NewHandler(server *Server) *Handler {
	return &Handler{
		server: server,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	v1 := e.Group("v1")
	v1.GET("nodes", h.handleListNodes)
	v1.POST("nodes", h.handleRegisterNode)
	v1.POST("nodes/:id/heartbeat", h.handleHeartbeat)
	v1.POST("jobs", h.handleRegisterJob)
}

func (h *Handler) handleListNodes(c *echo.Context) error {
	ctx := c.Request().Context()

	nodes := h.server.ListNodes(ctx)

	result := make(api.NodeListResponse, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, *h.convertNode(&node))
	}

	return c.JSON(200, result)
}

func (h *Handler) handleRegisterNode(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.NodeRegistrationRequest
	err := c.Bind(&request)
	if err != nil {
		return err
	}

	h.server.RegisterNode(ctx, &NodeRegistration{
		ID:     request.ID,
		Host:   request.Host,
		Port:   request.Port,
		CPU:    request.CPU,
		Memory: request.Memory,
		OS:     request.OS,
		Arch:   request.Arch,
	})

	return nil
}

func (h *Handler) handleHeartbeat(c *echo.Context) error {
	ctx := c.Request().Context()

	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = h.server.Heartbeat(ctx, uuid)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

func (h *Handler) handleRegisterJob(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.JobRegistrationRequest
	err := c.Bind(&request)
	if err != nil {
		return err
	}

	err = h.server.RegisterJob(ctx, &request.Spec)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

func (h *Handler) convertNode(node *Node) *api.NodeResponse {
	return &api.NodeResponse{
		ID:            node.ID,
		Host:          node.Host,
		Port:          node.Port,
		Status:        api.NodeStatusResponse(node.Status),
		LastHeartbeat: node.LastHeartbeat,
	}
}
