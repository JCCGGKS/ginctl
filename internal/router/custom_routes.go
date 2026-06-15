package router

import (
	"github.com/gin-gonic/gin"

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
