//go:build embed

package web

import "sync"

// HTMLCache 缓存注入了公开设置的 index.html（CSP nonce 占位符在每次请求时替换）。
//
// 不发 ETag、不回 304：页面里内联配置脚本的 nonce 每次请求都换，浏览器拿 304 沿用旧页面时，
// 旧 nonce 对不上新的 CSP 头，脚本会被拦掉（2026-10-04 UI E2E 发现）。
type HTMLCache struct {
	mu         sync.RWMutex
	cachedHTML []byte
}

// NewHTMLCache creates a new HTML cache instance
func NewHTMLCache() *HTMLCache {
	return &HTMLCache{}
}

// Invalidate marks the cache as stale
func (c *HTMLCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedHTML = nil
}

// Get returns the cached HTML or nil if cache is stale
func (c *HTMLCache) Get() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cachedHTML
}

// Set updates the cache with new rendered HTML
func (c *HTMLCache) Set(html []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedHTML = html
}
