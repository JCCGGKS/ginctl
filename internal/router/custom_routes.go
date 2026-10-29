package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"
)

// CustomRoute describes one custom HTTP route inside a group.
type CustomRoute struct {
	Method  string
	Path    string
	Handler func(*svc.ServiceContext) gin.HandlerFunc
}

// CustomRouteGroup bundles routes that share a common path prefix and optional
// group middleware.
type CustomRouteGroup struct {
	Prefix      string
	Middlewares []func(*svc.ServiceContext) gin.HandlerFunc
	Routes      []CustomRoute
}

var customGroups []CustomRouteGroup

// RegisterCustomRoutes appends a prefixed group of routes to the global registry.
func RegisterCustomRoutes(group CustomRouteGroup) {
	customGroups = append(customGroups, group)
}

func registerCustomRoutes(engine *gin.Engine, svcCtx *svc.ServiceContext) {
	for _, group := range customGroups {
		rg := engine.Group(group.Prefix)
		for _, mw := range group.Middlewares {
			rg.Use(mw(svcCtx))
		}
		for _, route := range group.Routes {
			rg.Handle(route.Method, route.Path, route.Handler(svcCtx))
		}
	}
}

// init registers the skeleton's built-in routes. Kept here so the registry is
// the single source of route definitions.
func init() {
	RegisterCustomRoutes(CustomRouteGroup{
		Prefix: "",
		Routes: []CustomRoute{
			{
				Method: http.MethodGet,
				Path:   "/healthz",
				Handler: func(_ *svc.ServiceContext) gin.HandlerFunc {
					return handler.Health
				},
			},
		},
	})

	RegisterCustomRoutes(CustomRouteGroup{
		Prefix: "/api/v1",
		Routes: []CustomRoute{
			{
				Method: http.MethodPost,
				Path:   "/login",
				Handler: func(svcCtx *svc.ServiceContext) gin.HandlerFunc {
					return handler.NewAuthHandler(svcCtx).Login
				},
			},
		},
	})

	RegisterCustomRoutes(CustomRouteGroup{
		Prefix: "/api/v1",
		Middlewares: []func(*svc.ServiceContext) gin.HandlerFunc{
			func(svcCtx *svc.ServiceContext) gin.HandlerFunc {
				return middleware.Auth(svcCtx.TokenManager)
			},
		},
		Routes: []CustomRoute{
			{
				Method: http.MethodGet,
				Path:   "/me",
				Handler: func(svcCtx *svc.ServiceContext) gin.HandlerFunc {
					return handler.NewAuthHandler(svcCtx).Me
				},
			},
		},
	})
}
