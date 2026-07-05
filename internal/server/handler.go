package server

import (
	"github.com/labstack/echo/v5"
)

type Handler struct {
	// server *Server
}

func NewHandler() *Handler {
	// return &Handler{
	// 	server: server,
	// }

	return &Handler{}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/nodes", h.handleListNodes)
	e.POST("/nodes", h.handleRegisterNode)
	e.POST("/heartbeats", h.handleHeartbeat)
}

func (h *Handler) handleListNodes(c *echo.Context) error {
	ctx := c.Request().Context()

	return nil
}

func (h *Handler) handleRegisterNode(c *echo.Context) error {
	ctx := c.Request().Context()

	return nil
}

func (h *Handler) handleHeartbeat(c *echo.Context) error {
	ctx := c.Request().Context()

	return nil
}
