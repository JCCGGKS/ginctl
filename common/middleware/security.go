package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders applies conservative defaults for a JSON API.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 禁止浏览器猜测响应类型，降低 MIME 混淆攻击。
		c.Header("X-Content-Type-Options", "nosniff")
		// 禁止页面被 iframe 嵌入，防点击劫持。
		c.Header("X-Frame-Options", "DENY")
		// 跳转或加载子资源时不发送来源地址，防 Referer 泄露。
		c.Header("Referrer-Policy", "no-referrer")
		// JSON API 不加载外部资源，并禁止被嵌入。
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		// 禁止浏览器和中间代理缓存响应，避免敏感数据落盘。
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
