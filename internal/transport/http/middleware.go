// Package httpapi 传输层基石：中间件链 + 健康探针 + 统一错误体。
// 业务路由继续由 internal/arena 提供（向后兼容），本包只做正交包装。
package httpapi

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/observability"
)

type ctxKey string

const traceKey ctxKey = "traceId"

// maxAPIBodyBytes caps every JSON API request. Source uploads are limited far
// below this by application policy; the transport guard exists so an
// unauthenticated or buggy client cannot stream an unbounded body into memory.
const maxAPIBodyBytes = 12 << 20

func newTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "trace-local"
	}
	return hex.EncodeToString(b[:])
}

// WithMiddleware 组装 requestID + recover + 访问日志。
func WithMiddleware(
	next http.Handler,
	log *slog.Logger,
	trustProxy func() bool,
	metrics *observability.Metrics,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusWriter{ResponseWriter: w}
		if metrics != nil {
			metrics.IncInFlight()
		}
		start := time.Now()
		defer func() {
			if metrics != nil {
				metrics.DecInFlight()
			}
		}()
		tid := newTraceID()
		ctx := context.WithValue(r.Context(), traceKey, tid)
		w.Header().Set("X-Trace-Id", tid)
		// The SPA and API are same-origin. These headers harden the standalone
		// binary without requiring a reverse proxy or external web server.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"base-uri 'self'; "+
				"object-src 'none'; "+
				"frame-ancestors 'none'; "+
				"form-action 'self'; "+
				"script-src 'self' blob: 'wasm-unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"worker-src 'self' blob:; "+
				"manifest-src 'self'")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		if isHTTPS(r, trustProxy != nil && trustProxy()) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if r.URL.Path == "/api/v1/" || strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead &&
			r.Method != http.MethodOptions && !sameOriginMutation(r) {
			WriteError(w, r, http.StatusForbidden, "cross_origin_request",
				"cross-origin state changes are not allowed")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/") && r.Body != nil &&
			r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Body = http.MaxBytesReader(w, r.Body, maxAPIBodyBytes)
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered", "traceId", tid, "panic", recovered)
				if recorder.status == 0 {
					WriteError(recorder, r, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}
			duration := time.Since(start)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			if metrics != nil {
				metrics.Observe(r.Method, observability.NormalizeRoute(r.URL.Path), status, duration)
			}
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"traceId", tid,
				"dur", duration.String(),
			)
		}()
		next.ServeHTTP(recorder, r.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *statusWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func isHTTPS(r *http.Request, trustProxy bool) bool {
	return r.TLS != nil ||
		(trustProxy && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https"))
}

// sameOriginMutation is defense in depth behind SameSite=Strict cookies.
// Non-browser API clients normally omit Origin and remain supported.
func sameOriginMutation(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// Healthz reports that the process is alive. Readiness is handled by Server
// because it needs access to the configured persistence port.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// WriteError 统一 ProblemDetails 风格错误体。
func WriteError(w http.ResponseWriter, r *http.Request, code int, errCode, msg string) {
	tid, _ := r.Context().Value(traceKey).(string)
	writeJSON(w, code, map[string]any{"code": errCode, "message": msg, "traceId": tid})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
