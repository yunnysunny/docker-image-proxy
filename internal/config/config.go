package config

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Account struct {
	Username string
	Password string
}

type Config struct {
	// 服务器配置
	Port int
	// 上游Docker Registry配置
	UpstreamRegistry string
	// 缓存配置
	CacheDir string
	// 认证配置
	UpstreamAuthService string // 认证服务地址
	// 当前镜像服务地址
	SelfRegistry string
	SelfAuthService string // 当前镜像鉴权服务
	// 账号配置
	Accounts []string
	// 跳过认证代理
	SkipAuthProxy bool
	// 服务端加密密钥
	ServerSecret string
}

func NewConfig() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	accounts := []string{}
	accountsStr := getEnv("ACCOUNTS", "")
	if accountsStr != "" {
		accountStrs := strings.Split(accountsStr, ",")
		for _, accountStr := range accountStrs {
			account := base64.StdEncoding.EncodeToString([]byte(accountStr))
			accounts = append(accounts, account)
		}
	}
	return &Config{
		Port:                port,
		UpstreamRegistry:    getEnv("UPSTREAM_REGISTRY", "https://registry-1.docker.io"),
		CacheDir:            getEnv("CACHE_DIR", "./cache"),
		UpstreamAuthService: getEnv("AUTH_SERVICE", "https://auth.docker.io"),
		SelfRegistry:        getEnv("SELF_REGISTRY", "http://localhost:8080"),
		SelfAuthService:     "docker-image-proxy",
		SkipAuthProxy:       getEnv("SKIP_AUTH_PROXY", "false") == "true",
		Accounts:            accounts,
		ServerSecret:        getEnv("SERVER_SECRET", uuid.New().String()),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
