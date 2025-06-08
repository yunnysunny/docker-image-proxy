package middleware

import (
	// "fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yunnysunny/docker-image-proxy/internal/config"
	"github.com/yunnysunny/docker-image-proxy/internal/service"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	log     *logrus.Logger
	config  *config.Config
	service *service.TokenService
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(
	log *logrus.Logger,
	config *config.Config,
	service *service.TokenService,
) *AuthMiddleware {
	return &AuthMiddleware{
		log:     log,
		config:  config,
		service: service,
	}
}

// AuthRequired 需要认证的中间件处理函数
func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过认证的路由
		if c.Request.URL.Path == "/v2/" || c.Request.URL.Path == "/v2/auth" {
			c.Next()
			return
		}
		if m.config.SkipAuthProxy {// 本服务没有改写过鉴权，跳过中间件
			c.Next()
			return
		}

		// 获取认证头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.log.Warn("Missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization header",
			})
			c.Abort()
			return
		}

		// 解析认证头
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.log.Warn("Invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		// 验证token
		token := parts[1]
		claims, err := m.service.GetUnverifiedToken(token)
		if err != nil {
			m.log.WithError(err).Warn("Invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}
		if claims.Iss != m.config.SelfAuthService {//不是当前服务签名的TOKEN，跳过中间件
			c.Next()
			return
		}
		claims, err = m.service.GetToken(token)
		if err != nil {
			m.log.WithError(err).Warn("Invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		// 检查权限
		// if !m.checkScope(claims, c) {
		// 	m.log.WithFields(logrus.Fields{
		// 		"path":  c.Request.URL.Path,
		// 		"scope": claims.Scope,
		// 	}).Warn("Insufficient permissions")
		// 	c.JSON(http.StatusForbidden, gin.H{
		// 		"error": "Insufficient permissions",
		// 	})
		// 	c.Abort()
		// 	return
		// }

		// 将claims信息存储到上下文中
		c.Set("token", claims)
		c.Next()
	}
}

// checkScope 检查请求的权限范围
// func (m *AuthMiddleware) checkScope(claims *service.Token, c *gin.Context) bool {
// 	// 解析请求路径
// 	path := c.Request.URL.Path
// 	parts := strings.Split(path, "/")
// 	if len(parts) < 4 {
// 		return false
// 	}

// 	// 获取请求的操作类型
// 	var action string
// 	switch {
// 	case strings.HasSuffix(path, "/tags/list"):
// 		action = "pull"
// 	case strings.HasSuffix(path, "/manifests/"):
// 		action = "pull"
// 	case strings.HasSuffix(path, "/blobs/"):
// 		action = "pull"
// 	default:
// 		return false
// 	}

// 	// 构建请求的scope
// 	requestScope := fmt.Sprintf("repository:%s:%s", parts[2], action)

// 	// 检查scope是否匹配
// 	for _, scope := range claims.Scope {
// 		if scope == requestScope {
// 			return true
// 		}
// 	}

// 	return false
// }
