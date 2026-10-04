// Package httpapi 传输层基石：中间件链 + 健康探针 + 统一错误体。
// 业务路由继续由 internal/arena 提供（向后兼容），本包只做正交包装。
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey string

const traceKey ctxKey = "traceId"

func newTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "trace-local"
	}
	return hex.EncodeToString(b[:])
}

// WithMiddleware 组装 requestID + recover + 访问日志。
func WithMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := newTraceID()
		ctx := context.WithValue(r.Context(), traceKey, tid)
		w.Header().Set("X-Trace-Id", tid)
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "traceId", tid, "panic", rec)
				WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			log.Info("http", "method", r.Method, "path", r.URL.Path, "traceId", tid, "dur", time.Since(start).String())
		}()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Healthz 存活探针，Readyz 就绪探针（地基版直接就绪，sqlite 接入后检查迁移水位）。
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func Readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
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
