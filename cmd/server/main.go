package main

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yunnysunny/docker-image-proxy/internal/config"
	"github.com/yunnysunny/docker-image-proxy/internal/handler"
)

func main() {
	// 初始化日志
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 加载配置
	cfg := config.NewConfig()

	// 初始化处理器
	registryHandler := handler.NewRegistryHandler(log, cfg)

	// 设置路由
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Docker Registry API v2 路由
	v2 := r.Group("/v2")
	{
		// 认证相关路由
		v2.GET("/", registryHandler.HandleAuthChallenge)
		v2.GET("/auth", registryHandler.HandleAuth)

		// 镜像操作路由
		v2.GET("/_catalog", registryHandler.HandleCatalog)
		v2.GET("/:name/tags/list", registryHandler.HandleTags)
		v2.GET("/:name/manifests/:reference", registryHandler.HandleManifest)
		v2.GET("/:name/blobs/:digest", registryHandler.HandleBlob)
	}

	// 启动服务器
	addr := ":" + strconv.Itoa(cfg.Port)
	log.Infof("Starting Docker Registry Proxy on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}


