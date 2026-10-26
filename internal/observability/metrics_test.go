package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeRouteBoundsCardinality(t *testing.T) {
	tests := map[string]string{
		"/healthz":                            "healthz",
		"/readyz":                             "readyz",
		"/app/contest/cnt-42":                 "/app/*",
		"/assets/index-abc123.js":             "/assets/*",
		"/api/v1/contests":                    "/api/v1/contests",
		"/api/v1/contests/cnt-42":             "/api/v1/contests/:cid",
		"/api/v1/contests/cnt-42/submissions": "/api/v1/contests/:cid/submissions",
		"/api/v1/contests/cnt-42/problems/prb-7/files/a01.in": "/api/v1/contests/:cid/problems/:pid/files/:file",
		"/api/v1/users/usr-1/password":                        "/api/v1/users/:uid/password",
		"/api/v1/auth/login":                                  "/api/v1/auth/login",
		"/unknown/dynamic/path":                               "other",
	}
	for path, want := range tests {
		if got := NormalizeRoute(path); got != want {
			t.Errorf("NormalizeRoute(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestMetricsHandlerExportsCountersHistogramsAndInflight(t *testing.T) {
	metrics := NewMetrics()
	metrics.SetRuntimeStatsProvider(func() RuntimeStats {
		return RuntimeStats{
			JudgeWorkers:       4,
			JudgeJobsActive:    3,
			JudgeJobsQueued:    2,
			JudgeQueueCapacity: 128,
			SSESubscriberCount: 7,
		}
	})
	metrics.IncJudgeEnqueueFailure()
	metrics.IncInFlight()
	metrics.Observe(http.MethodGet, "/api/v1/contests/:cid", http.StatusOK, 25*time.Millisecond)
	metrics.DecInFlight()

	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		`forge_http_requests_in_flight 0`,
		`forge_http_requests_total{method="GET",route="/api/v1/contests/:cid",status="200"} 1`,
		`forge_http_request_duration_seconds_count{method="GET",route="/api/v1/contests/:cid",status="200"} 1`,
		`forge_http_request_duration_seconds_sum{method="GET",route="/api/v1/contests/:cid",status="200"} 0.025`,
		`forge_judge_workers 4`,
		`forge_judge_jobs_active 3`,
		`forge_judge_jobs_queued 2`,
		`forge_judge_queue_capacity 128`,
		`forge_judge_enqueue_failures_total 1`,
		`forge_sse_subscribers 7`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q:\n%s", want, body)
		}
	}
}

func TestRecordJudgeVerdictsExposedViaMetrics(t *testing.T) {
	RecordJudgeVerdicts(map[string]int{"AC": 3, "wa": 2}, 150)
	RecordJudgeVerdicts(map[string]int{"AC": 1}, 50)

	metrics := NewMetrics()
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	resp := recorder.Body.String()
	for _, want := range []string{
		`forge_judge_verdict_cases_total{verdict="ac"} 4`,
		`forge_judge_verdict_cases_total{verdict="wa"} 2`,
		"forge_judge_duration_count 2",
		"forge_judge_duration_sum_ms 200",
	} {
		if !strings.Contains(resp, want) {
			t.Fatalf("metrics missing %q\n%s", want, resp)
		}
	}
}
