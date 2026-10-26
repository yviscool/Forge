// Package observability provides dependency-free production signals.
package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

var durationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
}

type requestKey struct {
	method string
	route  string
	status int
}

type requestMetric struct {
	count   uint64
	sum     float64
	buckets []uint64
}

type Metrics struct {
	mu                   sync.Mutex
	requests             map[requestKey]*requestMetric
	runtimeStats         func() RuntimeStats
	inFlight             atomic.Int64
	judgeEnqueueFailures atomic.Uint64
}

// 判题 verdict 计数与耗时(池 sink 写入,渲染读出;包级以便池与
// HTTP server 各自持有实例时共享同一份计数)。
var (
	judgeVerdictCounters sync.Map // string(规范 verdict) -> *atomic.Uint64
	judgeDurationCount   atomic.Uint64
	judgeDurationSumMs   atomic.Uint64
	judgeDurationBuckets = newJudgeDurationBuckets()
	judgeWallStatuses    sync.Map // kind(lowbox/bwrap/cgroup) -> string 状态
)

// judgeDurationBounds 直方图上界(毫秒):P50/P95 尾部可见。
var judgeDurationBounds = []int64{50, 100, 250, 500, 1000, 2500, 5000, 10000}

func newJudgeDurationBuckets() []atomic.Uint64 {
	return make([]atomic.Uint64, len(judgeDurationBounds)+1) // 末桶 +Inf
}

// SetJudgeWallStatus 记录一平台资源墙/沙箱的探测状态(启动期调用)。
func SetJudgeWallStatus(kind, state string) {
	judgeWallStatuses.Store(kind, state)
}

// RecordJudgeVerdicts 记录一单评测的分点 verdict 计数与总耗时。
// 并发安全;键为 CaseVerdict 原文(AC/WA/TLE/...)。
func RecordJudgeVerdicts(verdicts map[string]int, durationMs int64) {
	for verdict, n := range verdicts {
		key := strings.ToLower(strings.TrimSpace(verdict))
		v, _ := judgeVerdictCounters.LoadOrStore(key, &atomic.Uint64{})
		v.(*atomic.Uint64).Add(uint64(n))
	}
	judgeDurationCount.Add(1)
	if durationMs > 0 {
		judgeDurationSumMs.Add(uint64(durationMs))
	}
	for i, bound := range judgeDurationBounds {
		if durationMs <= bound {
			judgeDurationBuckets[i].Add(1)
			return
		}
	}
	judgeDurationBuckets[len(judgeDurationBounds)].Add(1)
}

// RuntimeStats is a low-cardinality operational snapshot supplied by the
// composition root. Callbacks must be cheap and concurrency-safe.
type RuntimeStats struct {
	JudgeWorkers       int
	JudgeJobsActive    int
	JudgeJobsQueued    int
	JudgeQueueCapacity int
	SSESubscriberCount int
	JudgeCacheHits     uint64
	JudgeCacheMisses   uint64
}

func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[requestKey]*requestMetric)}
}

func (m *Metrics) SetRuntimeStatsProvider(provider func() RuntimeStats) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.runtimeStats = provider
	m.mu.Unlock()
}

func (m *Metrics) IncJudgeEnqueueFailure() {
	if m != nil {
		m.judgeEnqueueFailures.Add(1)
	}
}

func (m *Metrics) IncInFlight() {
	if m != nil {
		m.inFlight.Add(1)
	}
}

func (m *Metrics) DecInFlight() {
	if m != nil {
		m.inFlight.Add(-1)
	}
}

func (m *Metrics) Observe(method, route string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	key := requestKey{
		method: strings.ToUpper(method),
		route:  route,
		status: status,
	}
	seconds := duration.Seconds()
	m.mu.Lock()
	metric := m.requests[key]
	if metric == nil {
		metric = &requestMetric{buckets: make([]uint64, len(durationBuckets))}
		m.requests[key] = metric
	}
	metric.count++
	metric.sum += seconds
	for index, upper := range durationBuckets {
		if seconds <= upper {
			metric.buckets[index]++
		}
	}
	m.mu.Unlock()
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", metricsContentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(m.render()))
	})
}

func (m *Metrics) render() string {
	type row struct {
		key    requestKey
		metric requestMetric
	}

	m.mu.Lock()
	runtimeProvider := m.runtimeStats
	rows := make([]row, 0, len(m.requests))
	for key, metric := range m.requests {
		copied := requestMetric{
			count:   metric.count,
			sum:     metric.sum,
			buckets: append([]uint64(nil), metric.buckets...),
		}
		rows = append(rows, row{key: key, metric: copied})
	}
	m.mu.Unlock()
	var runtimeStats RuntimeStats
	if runtimeProvider != nil {
		runtimeStats = runtimeProvider()
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].key.route != rows[j].key.route {
			return rows[i].key.route < rows[j].key.route
		}
		if rows[i].key.method != rows[j].key.method {
			return rows[i].key.method < rows[j].key.method
		}
		return rows[i].key.status < rows[j].key.status
	})

	var body strings.Builder
	body.WriteString("# HELP forge_http_requests_in_flight Current HTTP requests being served.\n")
	body.WriteString("# TYPE forge_http_requests_in_flight gauge\n")
	fmt.Fprintf(&body, "forge_http_requests_in_flight %d\n", m.inFlight.Load())
	body.WriteString("# HELP forge_http_requests_total Completed HTTP requests.\n")
	body.WriteString("# TYPE forge_http_requests_total counter\n")
	for _, row := range rows {
		labels := metricLabels(row.key)
		fmt.Fprintf(&body, "forge_http_requests_total{%s} %d\n", labels, row.metric.count)
	}
	body.WriteString("# HELP forge_http_request_duration_seconds HTTP request latency.\n")
	body.WriteString("# TYPE forge_http_request_duration_seconds histogram\n")
	for _, row := range rows {
		labels := metricLabels(row.key)
		for index, upper := range durationBuckets {
			fmt.Fprintf(
				&body,
				"forge_http_request_duration_seconds_bucket{%s,le=\"%s\"} %d\n",
				labels,
				strconv.FormatFloat(upper, 'g', -1, 64),
				row.metric.buckets[index],
			)
		}
		fmt.Fprintf(
			&body,
			"forge_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n",
			labels,
			row.metric.count,
		)
		fmt.Fprintf(
			&body,
			"forge_http_request_duration_seconds_sum{%s} %s\n",
			labels,
			strconv.FormatFloat(row.metric.sum, 'g', -1, 64),
		)
		fmt.Fprintf(
			&body,
			"forge_http_request_duration_seconds_count{%s} %d\n",
			labels,
			row.metric.count,
		)
	}
	body.WriteString("# HELP forge_judge_workers Configured judge worker concurrency.\n")
	body.WriteString("# TYPE forge_judge_workers gauge\n")
	fmt.Fprintf(&body, "forge_judge_workers %d\n", runtimeStats.JudgeWorkers)
	body.WriteString("# HELP forge_judge_jobs_active Judge jobs queued or running.\n")
	body.WriteString("# TYPE forge_judge_jobs_active gauge\n")
	fmt.Fprintf(&body, "forge_judge_jobs_active %d\n", runtimeStats.JudgeJobsActive)
	body.WriteString("# HELP forge_judge_jobs_queued Judge jobs waiting for a worker.\n")
	body.WriteString("# TYPE forge_judge_jobs_queued gauge\n")
	fmt.Fprintf(&body, "forge_judge_jobs_queued %d\n", runtimeStats.JudgeJobsQueued)
	body.WriteString("# HELP forge_judge_queue_capacity Maximum waiting judge jobs.\n")
	body.WriteString("# TYPE forge_judge_queue_capacity gauge\n")
	fmt.Fprintf(&body, "forge_judge_queue_capacity %d\n", runtimeStats.JudgeQueueCapacity)
	body.WriteString("# HELP forge_judge_enqueue_failures_total Transient judge handoff failures.\n")
	body.WriteString("# TYPE forge_judge_enqueue_failures_total counter\n")
	fmt.Fprintf(&body, "forge_judge_enqueue_failures_total %d\n", m.judgeEnqueueFailures.Load())
	body.WriteString("# HELP forge_judge_cache_hits_total Compile cache hits.\n")
	body.WriteString("# TYPE forge_judge_cache_hits_total counter\n")
	fmt.Fprintf(&body, "forge_judge_cache_hits_total %d\n", runtimeStats.JudgeCacheHits)
	body.WriteString("# HELP forge_judge_cache_misses_total Compile cache misses.\n")
	body.WriteString("# TYPE forge_judge_cache_misses_total counter\n")
	fmt.Fprintf(&body, "forge_judge_cache_misses_total %d\n", runtimeStats.JudgeCacheMisses)
	judgeVerdictCounters.Range(func(key, value any) bool {
		v := value.(*atomic.Uint64)
		verdict := escapeLabel(key.(string))
		body.WriteString("# HELP forge_judge_verdict_cases_total Judged cases by verdict.\n")
		body.WriteString("# TYPE forge_judge_verdict_cases_total counter\n")
		fmt.Fprintf(&body, "forge_judge_verdict_cases_total{verdict=%q} %d\n", verdict, v.Load())
		return true
	})
	body.WriteString("# HELP forge_judge_duration_count Judge jobs completed.\n")
	body.WriteString("# TYPE forge_judge_duration_count counter\n")
	fmt.Fprintf(&body, "forge_judge_duration_count %d\n", judgeDurationCount.Load())
	body.WriteString("# HELP forge_judge_duration_sum_ms Total judge job duration in ms.\n")
	body.WriteString("# TYPE forge_judge_duration_sum_ms counter\n")
	fmt.Fprintf(&body, "forge_judge_duration_sum_ms %d\n", judgeDurationSumMs.Load())
	cumulative := uint64(0)
	for i, bound := range judgeDurationBounds {
		cumulative += judgeDurationBuckets[i].Load()
		body.WriteString("# HELP forge_judge_duration_bucket Judge job duration by upper bound (cumulative).\n")
		body.WriteString("# TYPE forge_judge_duration_bucket counter\n")
		fmt.Fprintf(&body, "forge_judge_duration_bucket{le=%q} %d\n", strconv.FormatInt(bound, 10), cumulative)
	}
	body.WriteString("# HELP forge_judge_duration_bucket Judge job duration by upper bound (cumulative).\n")
	body.WriteString("# TYPE forge_judge_duration_bucket counter\n")
	fmt.Fprintf(&body, "forge_judge_duration_bucket{le=%q} %d\n", "+Inf", judgeDurationCount.Load())
	judgeWallStatuses.Range(func(kind, state any) bool {
		body.WriteString("# HELP forge_judge_sandbox_status Judge wall/sandbox probe state per platform facility.\n")
		body.WriteString("# TYPE forge_judge_sandbox_status gauge\n")
		fmt.Fprintf(&body, "forge_judge_sandbox_status{kind=%q,state=%q} 1\n",
			escapeLabel(kind.(string)), escapeLabel(state.(string)))
		return true
	})
	body.WriteString("# HELP forge_sse_subscribers Active server-sent event streams.\n")
	body.WriteString("# TYPE forge_sse_subscribers gauge\n")
	fmt.Fprintf(&body, "forge_sse_subscribers %d\n", runtimeStats.SSESubscriberCount)
	return body.String()
}

func metricLabels(key requestKey) string {
	return fmt.Sprintf(
		`method="%s",route="%s",status="%d"`,
		escapeLabel(key.method),
		escapeLabel(key.route),
		key.status,
	)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func NormalizeRoute(path string) string {
	switch path {
	case "/healthz":
		return "healthz"
	case "/readyz":
		return "readyz"
	case "/metrics":
		return "metrics"
	}
	if path == "/" {
		return "root"
	}
	if path == "/app" || strings.HasPrefix(path, "/app/") {
		return "/app/*"
	}
	if strings.HasPrefix(path, "/assets/") {
		return "/assets/*"
	}
	if !strings.HasPrefix(path, "/api/v1/") {
		return "other"
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, "/api/v1/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "/api/v1"
	}
	switch parts[0] {
	case "auth":
		if len(parts) >= 2 {
			switch parts[1] {
			case "login", "logout", "me", "password":
				return "/api/v1/auth/" + parts[1]
			}
		}
		return "/api/v1/auth/*"
	case "events":
		if len(parts) >= 2 {
			switch parts[1] {
			case "ticket", "stream":
				return "/api/v1/events/" + parts[1]
			}
		}
		return "/api/v1/events/*"
	case "contests":
		return normalizeContestRoute(parts)
	case "users":
		if len(parts) == 1 {
			return "/api/v1/users"
		}
		if len(parts) >= 2 && (parts[1] == "import" || parts[1] == "export") {
			return "/api/v1/users/" + parts[1]
		}
		if len(parts) >= 3 && parts[2] == "password" {
			return "/api/v1/users/:uid/password"
		}
		return "/api/v1/users/:uid"
	case "groups":
		if len(parts) == 1 {
			return "/api/v1/groups"
		}
		if len(parts) >= 3 && parts[2] == "members" {
			return "/api/v1/groups/:gid/members"
		}
		return "/api/v1/groups/:gid"
	case "submissions":
		if len(parts) >= 3 {
			switch parts[2] {
			case "judge", "cases":
				return "/api/v1/submissions/:sid/" + parts[2]
			}
		}
		return "/api/v1/submissions/:sid"
	case "i18n":
		return "/api/v1/i18n"
	default:
		return "/api/v1/*"
	}
}

func normalizeContestRoute(parts []string) string {
	if len(parts) == 1 {
		return "/api/v1/contests"
	}
	if parts[1] == "import" {
		return "/api/v1/contests/import"
	}
	route := "/api/v1/contests/:cid"
	if len(parts) < 3 {
		return route
	}
	switch parts[2] {
	case "problems":
		route += "/problems"
		if len(parts) >= 4 {
			route += "/:pid"
		}
		if len(parts) >= 5 {
			switch parts[4] {
			case "files":
				route += "/files"
				if len(parts) >= 6 {
					route += "/:file"
				}
			case "validate", "rejudge":
				route += "/" + parts[4]
			}
		}
	case "submissions":
		route += "/submissions"
	case "ranking":
		route += "/ranking"
	case "messages":
		route += "/messages"
	case "settings":
		route += "/settings"
	case "participants":
		route += "/participants"
	case "groups":
		route += "/groups"
	case "start":
		route += "/start"
	case "finish":
		route += "/finish"
	case "bundle":
		route += "/bundle"
	case "export":
		route += "/export"
	case "pdf":
		route += "/pdf"
	case "unlock-zip":
		route += "/unlock-zip"
	case "statistics":
		route += "/statistics"
		if len(parts) >= 4 && parts[3] == "export" {
			route += "/export"
		}
	default:
		route += "/*"
	}
	return route
}

// RuntimeStatsSnapshot 返回运行时指标的即时快照（托盘速览面板用）。
func (m *Metrics) RuntimeStatsSnapshot() RuntimeStats {
	if m.runtimeStats == nil {
		return RuntimeStats{}
	}
	return m.runtimeStats()
}
