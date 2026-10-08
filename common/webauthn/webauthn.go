package webauthn

import (
	"github.com/modeltaps/modeltaps/common/config"
	"log"
	"net/url"
	"strings"
	"sync"

	"github.com/go-webauthn/webauthn/webauthn"
)

var w *webauthn.WebAuthn

// InitWebAuthn 初始化 WebAuthn 配置，用于在服务中启用 WebAuthn 身份验证功能。
// 如果初始化失败，将会记录致命错误，并终止程序运行。
func InitWebAuthn() error {
	rpID := extractRPID(config.ServerAddress)
	var err error
	w, err = webauthn.New(&webauthn.Config{
		RPDisplayName: config.SystemName,    // 显示名称
		RPID:          rpID,                 // FQDN
		RPOrigins:     buildRPOrigins(rpID), // 添加允许的源地址列表
	})
	if err != nil {
		log.Fatal("Failed to configure and create WebAuthn:", err)
		return err
	}
	return nil
}

// 确保线程安全
var mu sync.Mutex

// GetWebAuthn 返回 WebAuthn 实例，如果尚未初始化，则初始化并返回。
// 如果初始化失败，返回错误。
func GetWebAuthn() (*webauthn.WebAuthn, error) {
	mu.Lock()
	defer mu.Unlock()
	if w == nil {
		err := InitWebAuthn()
		if err != nil {
			return nil, err
		}
	}
	return w, nil
}

// buildRPOrigins 构建允许的源地址列表。
// 生产环境前端与后端同源，使用 ServerAddress 即可；
// 开发环境（rpID 为 localhost）前端由 vite 开发服务器在独立端口提供，
// 而 go-webauthn 的源校验包含端口，故需额外放行 vite 开发端口，
// 否则 finish 阶段会因 origin 不匹配被拒绝。
func buildRPOrigins(rpID string) []string {
	origins := []string{config.ServerAddress}
	if rpID == "localhost" {
		origins = append(origins, "http://localhost:5173", "http://localhost:3010")
	}
	return origins
}

// extractRPID 从服务器地址提取有效的 RPID
func extractRPID(serverAddress string) string {
	// 如果是 localhost，直接返回 localhost
	if strings.Contains(serverAddress, "localhost") {
		return "localhost"
	}

	// 解析URL获取主机名
	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	u, err := url.Parse(serverAddress)
	if err != nil {
		log.Printf("Failed to parse server address: %v, falling back to localhost", err)
		return "localhost"
	}

	// 移除端口号，只保留主机名
	host := u.Hostname()
	if host == "" {
		return "localhost"
	}

	return host
}
