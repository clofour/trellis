package server

import (
	"net/http"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/models"
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
	e.GET("/v1/nodes", h.handleListNodes)
	e.POST("/v1/nodes", h.handleRegisterNode)
	e.POST("/v1/nodes/:id/heartbeat", h.handleHeartbeat)
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

func (h *Handler) convertNode(node *models.Node) *api.NodeResponse {
	return &api.NodeResponse{
		ID:            node.ID,
		Host:          node.Host,
		Port:          node.Port,
		Status:        api.NodeStatusResponse(node.Status),
		LastHeartbeat: node.LastHeartbeat,
	}
}
