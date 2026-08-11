package websearch

import (
	"fmt"
	"net/http"
	"strings"
)

// EngineFactory 按引擎类型创建引擎实例
type EngineFactory struct {
	client *http.Client
}

// NewEngineFactory 创建引擎工厂（client 超时建议 15s，搜索为外部网络请求）
func NewEngineFactory(client *http.Client) *EngineFactory {
	return &EngineFactory{client: client}
}

// Create 按引擎类型返回引擎实例，未知类型返回错误
func (f *EngineFactory) Create(engineType string) (Engine, error) {
	switch strings.ToLower(strings.TrimSpace(engineType)) {
	case EngineDuckDuckGo:
		return newDuckDuckGoEngine(f.client), nil
	case EngineTavily:
		return newTavilyEngine(f.client), nil
	case EngineBaidu:
		return newBaiduEngine(f.client), nil
	default:
		return nil, fmt.Errorf("不支持的搜索引擎类型: %s", engineType)
	}
}

// SupportedEngines 支持的引擎类型列表（与 /types 接口元数据一致）
func SupportedEngines() []string {
	return []string{EngineDuckDuckGo, EngineTavily, EngineBaidu}
}
