package nodeapp

import (
	"context"
	"crypto/subtle"

	"github.com/clofour/trellis/internal/auth"
	"github.com/clofour/trellis/internal/server"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func clusterAuthMiddleware(token string) echo.MiddlewareFunc {
	return middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{KeyLookup: "header:Authorization:Bearer ", Validator: func(_ *echo.Context, key string, _ middleware.ExtractorSource) (bool, error) {
		return subtle.ConstantTimeCompare([]byte(key), []byte(token)) == 1, nil
	}})
}

func leaderAuthMiddleware(clusterToken string, tokenManager *auth.TokenManager) echo.MiddlewareFunc {
	return middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: "header:Authorization:Bearer ",
		Skipper: func(c *echo.Context) bool { return c.Request().URL.Path == "/metrics" },
		Validator: func(c *echo.Context, key string, _ middleware.ExtractorSource) (bool, error) {
			if subtle.ConstantTimeCompare([]byte(key), []byte(clusterToken)) == 1 {
				return true, nil
			}
			scope, err := tokenManager.ValidateToken(c.Request().Context(), key)
			if err != nil {
				return false, nil
			}
			if scope == nil {
				return false, nil
			}
			ctx := context.WithValue(c.Request().Context(), server.NamespaceContextKey, scope.Namespace)
			c.SetRequest(c.Request().WithContext(ctx))
			return true, nil
		},
	})
}
