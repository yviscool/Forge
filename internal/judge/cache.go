package judge

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CompileCache 编译产物缓存：同一（语言, 源码, 生效工具链, 编译器版本, 平台）
// 只编译一次。解释型语言不进缓存（无产物）。
//   - 键含编译器版本指纹与 GOOS/GOARCH：升级 g++ / 换平台后旧产物自动失效
//     （此前键只含 cpp 的 command/std/args，是正确性风险而不只是性能）；
//   - singleflight：同键并发只编译一次，其余等待后各自复制产物到 workdir；
//   - LRU：条数与字节双上限，超限淘汰最久未用条目（此前目录只增不减）；
//   - 命中复制到 workdir，多 worker 不并发执行同一文件。
type CompileCache struct {
	mu         sync.Mutex
	dir        string
	entries    map[string]*cacheEntry
	flights    map[string]*cacheFlight
	maxEntries int
	maxBytes   int64
	bytes      int64
	hits       int
	misses     int
	evictions  int

	fpMu sync.Mutex
	fp   map[string]string
}

type cacheEntry struct {
	atime int64
	size  int64
}

type cacheFlight struct {
	wg  sync.WaitGroup
	err error
}

const (
	defaultCacheMaxEntries = 256
	defaultCacheMaxBytes   = 1 << 30
)

// NewCompileCache 在 dir 下建缓存（dir 不存在则创建）。
func NewCompileCache(dir string) (*CompileCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &CompileCache{
		dir:        dir,
		entries:    map[string]*cacheEntry{},
		flights:    map[string]*cacheFlight{},
		maxEntries: defaultCacheMaxEntries,
		maxBytes:   defaultCacheMaxBytes,
		fp:         map[string]string{},
	}
	// 重启后重建 LRU 元数据（mtime 作初始访问时间，单调序保持）。
	if es, err := os.ReadDir(dir); err == nil {
		for _, e := range es {
			if e.IsDir() || filepath.Ext(e.Name()) != ".exe" {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			c.entries[strings.TrimSuffix(e.Name(), ".exe")] = &cacheEntry{
				atime: info.ModTime().UnixNano(),
				size:  info.Size(),
			}
			c.bytes += info.Size()
		}
	}
	return c, nil
}

// key 缓存键：lang + 源码 + 全量生效 ToolSpec + 编译器版本指纹 + GOOS/GOARCH。
func (c *CompileCache) key(lang string, src []byte, sp ToolSpec) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00", lang, runtime.GOOS, runtime.GOARCH)
	h.Write(src)
	fmt.Fprintf(h, "\x00cmd=%s\x00std=%s\x00args=%v\x00fp=%s",
		sp.Command, sp.Std, sp.Args, c.fingerprint(sp.Command))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// fingerprint 编译器版本指纹（`command --version` 首行，每命令探测一次）。
// 探测失败按空串处理：不阻塞评测，仅损失跨版本失效精度。
func (c *CompileCache) fingerprint(command string) string {
	if c == nil {
		return ""
	}
	c.fpMu.Lock()
	fp, ok := c.fp[command]
	c.fpMu.Unlock()
	if ok {
		return fp
	}
	fp = probeVersion(command)
	c.fpMu.Lock()
	c.fp[command] = fp
	c.fpMu.Unlock()
	return fp
}

func probeVersion(command string) string {
	arg := "--version"
	if command == "go" {
		arg = "version"
	}
	out, err := exec.Command(command, arg).CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if len(line) > 256 {
		line = line[:256]
	}
	return line
}

// execute 返回产物路径（已复制到 workdir）与是否缓存命中。
// 同键并发只放一个 goroutine 编译（singleflight），其余等待后各自复制。
func (c *CompileCache) execute(key, workdir string, compile func() (string, error)) (string, bool, error) {
	if c == nil {
		exe, err := compile()
		return exe, false, err
	}
	c.mu.Lock()
	if exe, ok := c.load(key, workdir); ok {
		c.mu.Unlock()
		return exe, true, nil
	}
	if fl, waiting := c.flights[key]; waiting {
		c.mu.Unlock()
		fl.wg.Wait()
		if fl.err != nil {
			return "", false, fl.err
		}
		c.mu.Lock()
		exe, ok := c.load(key, workdir)
		c.mu.Unlock()
		if !ok {
			return "", false, errors.New("compile cache entry vanished")
		}
		return exe, true, nil
	}
	fl := &cacheFlight{}
	fl.wg.Add(1)
	c.flights[key] = fl
	c.mu.Unlock()

	exe, err := compile()
	c.mu.Lock()
	delete(c.flights, key)
	if err == nil {
		c.store(key, exe)
	}
	fl.err = err
	c.mu.Unlock()
	fl.wg.Done()
	return exe, false, err
}

// load 命中则复制产物到 workdir 并刷新 LRU；未命中计 miss。
func (c *CompileCache) load(key, workdir string) (string, bool) {
	src := filepath.Join(c.dir, key+".exe")
	f, err := os.Open(src)
	if err != nil {
		c.misses++
		return "", false
	}
	defer f.Close()
	dst := filepath.Join(workdir, "cached.exe")
	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		c.misses++
		return "", false
	}
	_, copyErr := io.Copy(d, f)
	_ = d.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		c.misses++
		return "", false
	}
	if e := c.entries[key]; e != nil {
		e.atime = time.Now().UnixNano()
	}
	c.hits++
	return dst, true
}

// store 落盘产物并登记 LRU；超限淘汰最久未用条目（失败静默，只影响性能）。
func (c *CompileCache) store(key, exePath string) {
	b, err := os.ReadFile(exePath)
	if err != nil {
		return
	}
	dst := filepath.Join(c.dir, key+".exe")
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		return
	}
	if e := c.entries[key]; e != nil {
		c.bytes -= e.size
	}
	c.entries[key] = &cacheEntry{atime: time.Now().UnixNano(), size: int64(len(b))}
	c.bytes += int64(len(b))
	c.evict()
}

func (c *CompileCache) evict() {
	for len(c.entries) > c.maxEntries || (c.bytes > c.maxBytes && len(c.entries) > 1) {
		oldestKey := ""
		var oldest int64 = math.MaxInt64
		for k, e := range c.entries {
			if e.atime < oldest {
				oldest = e.atime
				oldestKey = k
			}
		}
		if oldestKey == "" {
			return
		}
		_ = os.Remove(filepath.Join(c.dir, oldestKey+".exe"))
		c.bytes -= c.entries[oldestKey].size
		delete(c.entries, oldestKey)
		c.evictions++
	}
}

// Stats 缓存命中率计数（观测用）。
func (c *CompileCache) Stats() (hits, misses uint64) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return uint64(c.hits), uint64(c.misses)
}
