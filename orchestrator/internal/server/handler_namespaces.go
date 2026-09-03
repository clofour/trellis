package server

import (
	"net/http"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/auth"
	"github.com/labstack/echo/v5"
)

func (h *Handler) handleListNamespaces(c *echo.Context) error {
	authz := authorization(c)
	if authz.scope == auth.AccessNamespace {
		return c.JSON(http.StatusOK, api.NamespaceListResponse{authz.namespace})
	}
	if !authz.root && authz.scope != auth.AccessCluster {
		return echo.NewHTTPError(http.StatusForbidden, "namespace discovery requires an authenticated scoped credential")
	}
	return c.JSON(http.StatusOK, h.server.ListNamespaces())
}
