package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yunnysunny/docker-image-proxy/internal/config"
)

type RegistryService struct {
	log    *logrus.Logger
	config *config.Config
	client *http.Client
}

func NewRegistryService(log *logrus.Logger, config *config.Config) *RegistryService {
	return &RegistryService{
		log:    log,
		config: config,
		client: &http.Client{},
	}
}

func (s *RegistryService) doGet(url string, c *gin.Context) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	_, exists := c.Get("token")

	for key, values := range c.Request.Header {
		if key == "Authorization" && exists {
			continue
		}
		req.Header[key] = values
	}
	resp, err := s.client.Do(req)
	return resp, err
}

// GetCatalog 从上游仓库获取镜像列表
func (s *RegistryService) GetCatalog(c *gin.Context) ([]string, error) {
	url := path.Join(s.config.UpstreamRegistry, "v2", "_catalog")
	resp, err := s.doGet(url, c)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Repositories, nil
}

// GetTags 从上游仓库获取镜像标签列表
func (s *RegistryService) GetTags(name string, c *gin.Context) ([]string, error) {
	url := path.Join(s.config.UpstreamRegistry, "v2", name, "tags", "list")
	resp, err := s.doGet(url, c)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Tags, nil
}

// GetManifest 从上游仓库获取镜像manifest
func (s *RegistryService) GetManifest(name, reference string, c *gin.Context) ([]byte, error) {
	url := path.Join(s.config.UpstreamRegistry, "v2", name, "manifests", reference)
	// req, err := http.NewRequest("GET", url, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create request: %v", err)
	// }

	// // 设置必要的请求头
	// req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := s.doGet(url, c)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetBlob 从上游仓库获取镜像层
func (s *RegistryService) GetBlob(name, digest string, c *gin.Context) (io.ReadCloser, error) {
	url := path.Join(s.config.UpstreamRegistry, "v2", name, "blobs", digest)
	resp, err := s.doGet(url, c)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// GetAuthChallenge 获取认证挑战信息
func (s *RegistryService) GetAuthChallenge() (*http.Response, error) {
	url := path.Join(s.config.UpstreamRegistry, "v2")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth challenge: %v", err)
	}

	return resp, nil
}

// Authenticate 处理认证请求
func (s *RegistryService) Authenticate(
	authHeader string, scope string, serviceName string,
) (string, error) {
	// 解析认证头
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	// 构建认证请求
	authURL := fmt.Sprintf("%s/token", s.config.UpstreamAuthService)
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %v", err)
	}

	// 设置查询参数
	q := req.URL.Query()
	q.Set("service", serviceName)
	q.Set("scope", scope)
	req.URL.RawQuery = q.Encode()

	// 设置认证头
	req.Header.Set("Authorization", authHeader)

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to authenticate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	// 解析响应
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %v", err)
	}

	return result.Token, nil
}

// func (s *RegistryService) _changeAuthChallenge(
// 	originalResp *http.Response, proxyURL string,
// ) (*http.Response, error) {

// }

// ModifyAuthChallenge 修改认证挑战信息
func (s *RegistryService) ModifyAuthChallenge(
	originalResp *http.Response, proxyURL string,
) (*http.Response, error) {
	// 获取原始的WWW-Authenticate头
	authHeader := originalResp.Header.Get("WWW-Authenticate")
	if authHeader == "" { // 源站不需要鉴权
		// 创建新的响应
		newResp := &http.Response{
			StatusCode: originalResp.StatusCode,
			Header:     make(http.Header),
			Body:       originalResp.Body,
		}

		// 复制原始响应头
		for k, v := range originalResp.Header {
			newResp.Header[k] = v
		}
		newResp.Header.Set("WWW-Authenticate", "Bearer realm=\""+proxyURL+"\",service=\""+s.config.SelfAuthService+"\"")

		return originalResp, nil
	}

	// 解析认证头
	// WWW-Authenticate: Bearer realm="xxx",service="xxx",scope="xxx"
	// 认证头格式举例：
	// WWW-Authenticate: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/ubuntu:pull"
	wwwAuthParts := strings.SplitN(authHeader, " ", 2)
	if len(wwwAuthParts) != 2 {
		return originalResp, nil
	}

	// 解析参数
	params := make(map[string]string)
	for _, param := range strings.Split(wwwAuthParts[1], ",") {
		kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			params[key] = value
		}
	}

	// 修改realm参数
	params["realm"] = proxyURL

	// 重建认证头
	newParams := make([]string, 0, len(params))
	for k, v := range params {
		newParams = append(newParams, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	newAuthHeader := fmt.Sprintf("%s %s", wwwAuthParts[0], strings.Join(newParams, ","))

	// 创建新的响应
	newResp := &http.Response{
		StatusCode: originalResp.StatusCode,
		Header:     make(http.Header),
		Body:       originalResp.Body,
	}

	// 复制原始响应头
	for k, v := range originalResp.Header {
		if k == "WWW-Authenticate" {
			newResp.Header.Set(k, newAuthHeader)
		} else {
			newResp.Header[k] = v
		}
	}

	return newResp, nil
}
