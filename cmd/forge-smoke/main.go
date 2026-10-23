// forge-smoke exercises a built Forge binary as a black box. It is used by the
// release pipeline after extracting the real distribution archive.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	adminPassword = "release-smoke-password-2026"
	contestName   = "Release Smoke Contest"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type serverProcess struct {
	cmd    *exec.Cmd
	logs   *lockedBuffer
	url    string
	data   string
	client *http.Client
}

func main() {
	binary := flag.String("binary", "", "path to the forge executable")
	flag.Parse()
	if strings.TrimSpace(*binary) == "" {
		fmt.Fprintln(os.Stderr, "-binary is required")
		os.Exit(2)
	}
	if err := run(*binary); err != nil {
		fmt.Fprintln(os.Stderr, "release smoke failed:", err)
		os.Exit(1)
	}
	fmt.Println("release smoke passed")
}

func run(binary string) error {
	absBinary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	dataDir, err := os.MkdirTemp("", "forge-release-smoke-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)

	srv, err := startServer(absBinary, dataDir)
	if err != nil {
		return err
	}
	if err := srv.login(); err != nil {
		srv.stop()
		return fmt.Errorf("first login: %w", err)
	}
	if err := srv.checkApp(); err != nil {
		srv.stop()
		return err
	}
	if err := srv.createContest(); err != nil {
		srv.stop()
		return err
	}
	srv.stop()

	restarted, err := startServer(absBinary, dataDir)
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	defer restarted.stop()
	if err := restarted.login(); err != nil {
		return fmt.Errorf("login after restart: %w", err)
	}
	if err := restarted.requireContest(); err != nil {
		return fmt.Errorf("sqlite persistence after restart: %w", err)
	}
	if err := restarted.requireLogoutRevocation(); err != nil {
		return err
	}
	return nil
}

func startServer(binary, dataDir string) (*serverProcess, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	logs := &lockedBuffer{}
	cmd := exec.Command(binary)
	cmd.Dir = dataDir
	cmd.Env = append(os.Environ(),
		"FORGE_ENV=development",
		"FORGE_ADDR="+addr,
		"FORGE_DATA_DIR="+dataDir,
		"FORGE_STORE=sqlite",
		"FORGE_ADMIN_PASSWORD="+adminPassword,
		"FORGE_AUTOJUDGE=0",
		"FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=0",
		"FORGE_MIN_FREE_DISK_MIB=0",
		"FORGE_TRUST_PROXY=0",
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start binary: %w", err)
	}
	jar, _ := cookiejar.New(nil)
	srv := &serverProcess{
		cmd:  cmd,
		logs: logs,
		url:  "http://" + addr,
		data: dataDir,
		client: &http.Client{
			Jar:     jar,
			Timeout: 8 * time.Second,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, path := range []string{"/healthz", "/readyz"} {
		if err := srv.waitForStatus(ctx, path, http.StatusOK); err != nil {
			srv.stop()
			return nil, fmt.Errorf("%s: %w\nserver logs:\n%s", path, err, logs.String())
		}
	}
	return srv, nil
}

func (s *serverProcess) stop() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
}

func (s *serverProcess) waitForStatus(ctx context.Context, path string, want int) error {
	var last error
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+path, nil)
		resp, err := s.client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if resp.StatusCode == want {
				return nil
			}
			last = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s: %w", path, last)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (s *serverProcess) login() error {
	code, body, err := s.requestJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"login":    "admin",
		"password": adminPassword,
	})
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("status %d: %s", code, body)
	}
	var session map[string]any
	if err := json.Unmarshal(body, &session); err != nil {
		return err
	}
	if _, exposed := session["token"]; exposed {
		return fmt.Errorf("login response exposed a bearer token")
	}
	return nil
}

func (s *serverProcess) checkApp() error {
	resp, err := s.client.Get(s.url + "/app")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("/app returned %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") ||
		!strings.Contains(string(body), `id="root"`) {
		return fmt.Errorf("/app did not serve the embedded SPA")
	}
	return nil
}

func (s *serverProcess) createContest() error {
	code, body, err := s.requestJSON(http.MethodPost, "/api/v1/contests", map[string]any{
		"name": contestName,
	})
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("create contest: status %d: %s", code, body)
	}
	return nil
}

func (s *serverProcess) requireContest() error {
	code, body, err := s.requestJSON(http.MethodGet, "/api/v1/contests", nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("list contests: status %d: %s", code, body)
	}
	var contests []map[string]any
	if err := json.Unmarshal(body, &contests); err != nil {
		return err
	}
	for _, contest := range contests {
		if contest["name"] == contestName {
			return nil
		}
	}
	return fmt.Errorf("created contest was not found after restart")
}

func (s *serverProcess) requireLogoutRevocation() error {
	code, body, err := s.requestJSON(http.MethodPost, "/api/v1/auth/logout", nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("logout: status %d: %s", code, body)
	}
	code, body, err = s.requestJSON(http.MethodGet, "/api/v1/contests", nil)
	if err != nil {
		return err
	}
	if code != http.StatusUnauthorized {
		return fmt.Errorf("old session survived logout: status %d: %s", code, body)
	}
	return nil
}

func (s *serverProcess) requestJSON(method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, s.url+path, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}
