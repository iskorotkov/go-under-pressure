package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
)

const pprofAuthHeader = "X-Pprof-Secret"

var errPprofUnauthorized = map[string]string{"error": "unauthorized"}

func PprofAuth(secret string) echo.MiddlewareFunc {
	secretBytes := []byte(secret)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if secret == "" {
				return next(c)
			}
			provided := c.Request().Header.Get(pprofAuthHeader)
			if subtle.ConstantTimeCompare([]byte(provided), secretBytes) != 1 {
				return c.JSON(http.StatusUnauthorized, errPprofUnauthorized)
			}
			return next(c)
		}
	}
}

func RegisterPprof(g *echo.Group) {
	g.GET("/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	g.GET("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	g.GET("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	g.GET("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.POST("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.GET("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	g.GET("/allocs", echo.WrapHandler(pprof.Handler("allocs")))
	g.GET("/block", echo.WrapHandler(pprof.Handler("block")))
	g.GET("/goroutine", echo.WrapHandler(pprof.Handler("goroutine")))
	g.GET("/heap", echo.WrapHandler(pprof.Handler("heap")))
	g.GET("/mutex", echo.WrapHandler(pprof.Handler("mutex")))
	g.GET("/threadcreate", echo.WrapHandler(pprof.Handler("threadcreate")))
}
