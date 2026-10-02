package vector

import (
	"ai-chat-service/pkg/config"
	"strings"
)

const ProviderKvstore = "kvstore"

// ProviderName 返回配置中的向量库 provider，统一小写；未配置时默认 kvstore。
func ProviderName(cnf *config.Config) string {
	if cnf == nil {
		return ProviderKvstore
	}
	name := strings.ToLower(strings.TrimSpace(cnf.Vector.Provider))
	if name == "" {
		return ProviderKvstore
	}
	return name
}
