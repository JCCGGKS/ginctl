package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/svc"
)

// CustomRoute describes one custom HTTP route inside a group.
type CustomRoute struct {
	Method  string
	Path    string
	Handler func(*svc.ServiceContext) gin.HandlerFunc
}

// CustomRouteGroup bundles routes that share a common path prefix. Prefix acts
// as the group name and is applied via gin's router group: Prefix "/api/v1" with
// a route Path "/users" registers "/api/v1/users". An empty Prefix mounts the
// routes at the root.
type CustomRouteGroup struct {
	Prefix string
	Routes []CustomRoute
}

var customGroups []CustomRouteGroup

// RegisterCustomRoutes appends a prefixed group of routes to the global registry.
func RegisterCustomRoutes(group CustomRouteGroup) {
	customGroups = append(customGroups, group)
}

func registerCustomRoutes(engine *gin.Engine, svcCtx *svc.ServiceContext) {
	for _, group := range customGroups {
		rg := engine.Group(group.Prefix)
		for _, route := range group.Routes {
			rg.Handle(route.Method, route.Path, route.Handler(svcCtx))
		}
	}
}

// 为避免循环依赖，将init放置在此
func init() {
	RegisterCustomRoutes(CustomRouteGroup{
		Prefix: "", // root: liveness probe sits at /healthz
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
}
