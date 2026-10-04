package judge

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CompileCache 编译产物缓存：同一源码+语言+工具链只编译一次。
// 解释型语言不进缓存（无产物）。
type CompileCache struct {
	mu   sync.Mutex
	dir  string
	hits int
}

// NewCompileCache 在 dir 下建缓存（dir 不存在则创建）。
func NewCompileCache(dir string) (*CompileCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &CompileCache{dir: dir}, nil
}

func cacheKey(lang string, src []byte, cfg ToolchainConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00", lang)
	h.Write(src)
	fmt.Fprintf(h, "\x00%s\x00%s\x00%s", cfg.CPP.Command, cfg.CPP.Std, cfg.CPP.Args)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get 命中返回可执行路径（复制到 workdir，避免多单并发执行同一文件 Liverace）。
func (c *CompileCache) Get(lang string, src []byte, cfg ToolchainConfig, workdir string) (string, bool) {
	if c == nil {
		return "", false
	}
	key := cacheKey(lang, src, cfg)
	cached := filepath.Join(c.dir, key+".exe")
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := os.ReadFile(cached)
	if err != nil {
		return "", false
	}
	dst := filepath.Join(workdir, "cached.exe")
	if err := os.WriteFile(dst, b, 0755); err != nil {
		return "", false
	}
	c.hits++
	return dst, true
}

// Put 存入缓存（失败不影响评测，只记 miss）。
func (c *CompileCache) Put(lang string, src []byte, cfg ToolchainConfig, exePath string) {
	if c == nil {
		return
	}
	b, err := os.ReadFile(exePath)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = os.WriteFile(filepath.Join(c.dir, cacheKey(lang, src, cfg)+".exe"), b, 0755)
}

// Hits 缓存命中次数（测试/观测用）。
func (c *CompileCache) Hits() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits
}
