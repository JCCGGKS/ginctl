package middleware

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gin-scaffold/common"

	"github.com/gin-gonic/gin"
)

var (
	defaultCORSMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	defaultCORSHeaders = []string{"Authorization", "Content-Type", "X-Request-Id"}
	defaultCORSExpose  = []string{"X-Request-Id"}
)

// CORS 使用到的 HTTP 头含义：
//   Origin（请求头）：浏览器跨域请求自动携带的源，格式为 scheme://host[:port]。
//   Access-Control-Request-Method（预检请求头）：浏览器声明实际请求要使用的方法。
//   Access-Control-Request-Headers（预检请求头）：浏览器声明实际请求要携带的请求头。
//   Access-Control-Allow-Origin（响应头）：服务端允许访问的来源；带凭据时不能使用 "*"。
//   Access-Control-Allow-Methods（响应头）：服务端允许跨域使用的实际请求方法。
//   Access-Control-Allow-Headers（响应头）：服务端允许跨域请求携带的请求头。
//   Access-Control-Allow-Credentials（响应头）：是否允许浏览器携带 Cookie、HTTP 认证等凭据。
//   Access-Control-Expose-Headers（响应头）：允许前端 JavaScript 读取的响应头。
//   Access-Control-Max-Age（响应头）：预检结果的缓存时间，单位为秒。
//   Vary（响应头）：提示缓存按 Origin 或预检请求头区分响应，避免缓存串用。

// CORSConfig is the transport-level CORS policy. Keep configuration decoding
// outside this package so common/middleware stays independent of internal/*.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

type corsPolicy struct {
	enabled             bool
	allowAllOrigins     bool
	allowedOrigins      map[string]struct{}
	allowedMethods      map[string]struct{}
	allowedHeaders      map[string]struct{}
	allowMethodsHeader  string
	allowHeadersHeader  string
	exposeHeadersHeader string
	allowCredentials    bool
	maxAgeSeconds       int
}

// CORS validates cross-origin requests against an exact origin allowlist. It is
// disabled when no origins are configured. Preflight requests are answered
// before routing; actual requests receive CORS response headers and continue.
func CORS(cfg CORSConfig, forbidden ErrorBuilder) gin.HandlerFunc {
	policy := newCORSPolicy(cfg)

	return func(c *gin.Context) {
		if !policy.enabled {
			c.Next()
			return
		}

		// Origin：当前请求的来源；没有该头说明不是浏览器跨域请求。
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}

		// Same-origin requests do not need CORS headers. Checking Host here also
		// avoids rejecting same-origin POST requests that include an Origin header.
		if sameOrigin(c.Request, origin) {
			c.Next()
			return
		}

		c.Writer.Header().Add("Vary", "Origin") // 响应内容随 Origin 变化，缓存必须按来源区分
		if !policy.originAllowed(origin) {
			common.Fail(c, forbidden("cors origin is not allowed"))
			return
		}

		// Access-Control-Request-Method 非空表示这是浏览器发起的 CORS 预检请求。
		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			if !policy.handlePreflight(c, origin) {
				common.Fail(c, forbidden("cors preflight is not allowed"))
			}
			return
		}
		if _, ok := policy.allowedMethods[c.Request.Method]; !ok {
			common.Fail(c, forbidden("cors method is not allowed"))
			return
		}

		policy.writeActualHeaders(c, origin)
		c.Next()
	}
}

func newCORSPolicy(cfg CORSConfig) corsPolicy {
	policy := corsPolicy{
		allowedOrigins: make(map[string]struct{}),
		allowedMethods: make(map[string]struct{}),
		allowedHeaders: make(map[string]struct{}),
	}

	for _, raw := range cfg.AllowOrigins {
		origin := normalizeOrigin(raw)
		if origin == "" {
			continue
		}
		if origin == "*" {
			policy.allowAllOrigins = true
			continue
		}
		policy.allowedOrigins[origin] = struct{}{}
	}

	// Empty allowOrigins disables CORS. A wildcard combined with credentials is
	// rejected by config validation; if it reaches here anyway, fail closed.
	if len(policy.allowedOrigins) == 0 && !policy.allowAllOrigins {
		return corsPolicy{}
	}
	policy.enabled = true
	if policy.allowAllOrigins && cfg.AllowCredentials {
		policy.allowAllOrigins = false
		policy.allowedOrigins = make(map[string]struct{})
	}

	methods := normalizeList(cfg.AllowMethods, defaultCORSMethods, true)
	for _, method := range methods {
		policy.allowedMethods[method] = struct{}{}
	}
	headers := normalizeList(cfg.AllowHeaders, defaultCORSHeaders, false)
	for _, header := range headers {
		policy.allowedHeaders[strings.ToLower(header)] = struct{}{}
	}
	expose := normalizeList(cfg.ExposeHeaders, defaultCORSExpose, false)

	policy.allowMethodsHeader = strings.Join(methods, ", ")
	policy.allowHeadersHeader = strings.Join(headers, ", ")
	policy.exposeHeadersHeader = strings.Join(expose, ", ")
	policy.allowCredentials = cfg.AllowCredentials
	policy.maxAgeSeconds = int(cfg.MaxAge / time.Second)
	if policy.maxAgeSeconds < 0 {
		policy.maxAgeSeconds = 0
	}
	return policy
}

func (p corsPolicy) originAllowed(origin string) bool {
	if normalizeOrigin(origin) == "" {
		return false
	}
	if p.allowAllOrigins {
		return true
	}
	_, ok := p.allowedOrigins[normalizeOrigin(origin)]
	return ok
}

func (p corsPolicy) handlePreflight(c *gin.Context, origin string) bool {
	// Access-Control-Request-Method：浏览器准备实际使用的 HTTP 方法。
	method := strings.ToUpper(strings.TrimSpace(c.GetHeader("Access-Control-Request-Method")))
	if _, ok := p.allowedMethods[method]; !ok {
		return false
	}
	// Access-Control-Request-Headers：浏览器准备实际携带的自定义请求头列表。
	if !p.headersAllowed(c.GetHeader("Access-Control-Request-Headers")) {
		return false
	}

	p.writeActualHeaders(c, origin)
	c.Header("Access-Control-Allow-Methods", p.allowMethodsHeader)    // 允许实际请求使用的方法
	c.Header("Access-Control-Allow-Headers", p.allowHeadersHeader)    // 允许实际请求携带的请求头
	c.Header("Access-Control-Max-Age", strconv.Itoa(p.maxAgeSeconds)) // 预检结果缓存时间（秒）
	c.Writer.Header().Add("Vary", "Access-Control-Request-Method")    // 预检响应随请求方法变化
	c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")   // 预检响应随请求头变化
	c.AbortWithStatus(http.StatusNoContent)
	return true
}

func (p corsPolicy) headersAllowed(raw string) bool {
	for _, header := range strings.Split(raw, ",") {
		header = strings.ToLower(strings.TrimSpace(header))
		if header == "" {
			continue
		}
		if _, ok := p.allowedHeaders[header]; !ok {
			return false
		}
	}
	return true
}

func (p corsPolicy) writeActualHeaders(c *gin.Context, origin string) {
	if p.allowAllOrigins {
		c.Header("Access-Control-Allow-Origin", "*") // 允许任意来源；仅在未启用凭据时有效
	} else {
		c.Header("Access-Control-Allow-Origin", origin) // 只回显白名单中的精确来源
	}
	if p.allowCredentials {
		c.Header("Access-Control-Allow-Credentials", "true") // 允许浏览器携带 Cookie/认证凭据
	}
	if p.exposeHeadersHeader != "" {
		c.Header("Access-Control-Expose-Headers", p.exposeHeadersHeader) // 允许前端读取指定响应头
	}
}

func sameOrigin(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func normalizeOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "*" {
		return "*"
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	if parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return scheme + "://" + strings.ToLower(parsed.Host)
}

func normalizeList(values, defaults []string, upper bool) []string {
	if len(values) == 0 {
		values = defaults
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if upper {
			value = strings.ToUpper(value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
