package handler

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yunnysunny/docker-image-proxy/internal/config"
	"github.com/yunnysunny/docker-image-proxy/internal/service"
)

type RegistryHandler struct {
	log     *logrus.Logger
	config  *config.Config
	service *service.RegistryService
	tokenService *service.TokenService
}

func NewRegistryHandler(log *logrus.Logger, config *config.Config) *RegistryHandler {
	return &RegistryHandler{
		log:     log,
		config:  config,
		service: service.NewRegistryService(log, config),
		tokenService: service.NewTokenService(log, config),
	}
}

// HandleCatalog 处理镜像仓库列表请求
func (h *RegistryHandler) HandleCatalog(c *gin.Context) {
	repositories, err := h.service.GetCatalog(c)
	if err != nil {
		h.log.WithError(err).Error("Failed to get catalog")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get catalog",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"repositories": repositories,
	})
}

// HandleTags 处理镜像标签列表请求
func (h *RegistryHandler) HandleTags(c *gin.Context) {
	name := c.Param("name")
	tags, err := h.service.GetTags(name, c)
	if err != nil {
		h.log.WithError(err).WithField("name", name).Error("Failed to get tags")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get tags",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name": name,
		"tags": tags,
	})
}

// HandleManifest 处理镜像manifest请求
func (h *RegistryHandler) HandleManifest(c *gin.Context) {
	name := c.Param("name")
	reference := c.Param("reference")

	manifest, err := h.service.GetManifest(name, reference, c)
	if err != nil {
		h.log.WithError(err).WithFields(logrus.Fields{
			"name":      name,
			"reference": reference,
		}).Error("Failed to get manifest")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get manifest",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	c.Data(http.StatusOK, "application/vnd.docker.distribution.manifest.v2+json", manifest)
}

// HandleBlob 处理镜像层请求
func (h *RegistryHandler) HandleBlob(c *gin.Context) {
	name := c.Param("name")
	digest := c.Param("digest")

	blob, err := h.service.GetBlob(name, digest, c)
	if err != nil {
		h.log.WithError(err).WithFields(logrus.Fields{
			"name":   name,
			"digest": digest,
		}).Error("Failed to get blob")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get blob",
		})
		return
	}
	defer blob.Close()

	// 流式传输blob数据
	io.Copy(c.Writer, blob)
}

func (h *RegistryHandler) HandleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	base64Auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	if len(h.config.Accounts) > 0 {// 如果配置了账号，则需要验证账号是否在配置中
		accountAllowed := false
		for _, acc := range h.config.Accounts {
			if acc == base64Auth {
				accountAllowed = true
				break
			}
		}
		if !accountAllowed {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized account",
			})
			return
		}
	}
	if h.config.UpstreamNoAuth {
		c.JSON(http.StatusOK, gin.H{
			"token": "test",
		})
		return
	}
	token, err := h.service.LoginUpstream(username, password)
	if err != nil {
		h.log.WithError(err).Error("Failed to get docker registry token")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get docker registry token",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}


// HandleAuthChallenge 处理认证挑战请求
func (h *RegistryHandler) HandleAuthChallenge(c *gin.Context) {
	// 获取上游认证挑战信息
	resp, err := h.service.GetAuthChallenge()
	if err != nil {
		h.log.WithError(err).Error("Failed to get auth challenge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get auth challenge",
		})
		return
	}
	defer resp.Body.Close()
	if h.config.SkipAuthProxy {// 不做改写
		io.Copy(c.Writer, resp.Body)
		return
	}

	// 修改认证挑战信息
	selfRegistry := h.config.SelfRegistry
	proxyURL, err := url.Parse(selfRegistry)
	if err != nil {
		io.Copy(c.Writer, resp.Body)
		return
	}
	proxyURL.Path = "/v2/auth"
	modifiedResp, err := h.service.ModifyAuthChallenge(resp, proxyURL.String())
	if err != nil {
		h.log.WithError(err).Error("Failed to modify auth challenge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to modify auth challenge",
		})
		return
	}

	// 复制响应头
	for k, v := range modifiedResp.Header {
		c.Header(k, v[0])
	}

	// 设置状态码
	c.Status(modifiedResp.StatusCode)

	// 复制响应体
	if modifiedResp.Body != nil {
		io.Copy(c.Writer, modifiedResp.Body)
	}
}

// HandleAuth 处理认证请求
func (h *RegistryHandler) HandleAuth(c *gin.Context) {
	// 获取认证头, 格式：Authorization: Bearer <token>
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header is required",
		})
		return
	}
	authToken := strings.SplitN(authHeader, " ", 2)
	if len(authToken) != 2 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header format",
		})
		return
	}

	if len(h.config.Accounts) > 0 {// 如果配置了账号，则需要验证账号是否在配置中
		accountAllowed := false
		for _, acc := range h.config.Accounts {
			if acc == authToken[1] {
				accountAllowed = true
				break
			}
		}
		if !accountAllowed {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized account",
			})
			return
		}
	}
	// 处理认证
	serviceName := c.Query("service")
	scope := c.Query("scope")
	var token string
	var err error
	if serviceName == h.config.SelfAuthService {// 源站没有认证服务
		token, err = h.tokenService.GetDockerRegistryToken(scope)
	} else {// 源站有认证服务
		token, err = h.service.Authenticate(authHeader, scope, serviceName)
	}

	if err != nil {
		h.log.WithError(err).Error("Authentication failed")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// 返回token
	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}
