package httpapi

import (
	"sort"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// TraySummaryContest 托盘速览的当前比赛卡片数据。
type TraySummaryContest struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Mode      string     `json:"mode"`
	Status    string     `json:"status"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
	// WindowEndsAt 考试窗口截止（含全局设置；个人加时在监考大屏里看）。
	WindowEndsAt *time.Time `json:"windowEndsAt,omitempty"`
}

// TraySummaryEvent 最近提交动态。
type TraySummaryEvent struct {
	UserName    string    `json:"userName"`
	ProblemCode string    `json:"problemCode"`
	Verdict     string    `json:"verdict"`
	SubmittedAt time.Time `json:"submittedAt"`
}

// TraySummary 托盘速览面板的一次快照：10 秒内回答
// 「正在考什么、还剩多久、有人掉线吗、判题堵没堵、有没有异常」。
type TraySummary struct {
	GeneratedAt  time.Time           `json:"generatedAt"`
	Version      string              `json:"version,omitempty"`
	Contest      *TraySummaryContest `json:"contest,omitempty"`
	Online       int                 `json:"online"`
	Participants int                 `json:"participants"`
	Submissions  int                 `json:"submissions"`
	Queued       int                 `json:"queued"`
	Conflicts    int                 `json:"conflicts"`
	Recent       []TraySummaryEvent  `json:"recent"`
}

// traySummary 组合监考快照与运行时指标。没有进行中的比赛时
// 只返回服务健康信息，面板据此显示「当前没有进行中的比赛」。
func (h *Server) traySummary() TraySummary {
	summary := TraySummary{
		GeneratedAt: time.Now(),
		Recent:      []TraySummaryEvent{},
	}

	contests := h.svc.ListContests()
	// 心流顺序：进行中的比赛 > 最近创建的比赛。
	sort.Slice(contests, func(i, j int) bool {
		a, b := contests[i], contests[j]
		if (a.Status == "running") != (b.Status == "running") {
			return a.Status == "running"
		}
		return a.CreatedAt.After(b.CreatedAt)
	})
	if len(contests) == 0 {
		return summary
	}
	c := contests[0]
	summary.Contest = &TraySummaryContest{
		ID: c.ID, Name: c.Name, Mode: domain.NormalizeMode(c.Mode), Status: c.Status,
	}
	if c.StartedAt.After(time.Time{}) {
		t := c.StartedAt
		summary.Contest.StartedAt = &t
	}
	if c.WindowEnabled && c.EndTime != nil {
		t := c.EndTime.Add(0)
		summary.Contest.WindowEndsAt = &t
	}

	presence := map[string]time.Time{}
	if h.presenceSource != nil {
		presence = h.presenceSource(c.ID)
	}
	snapshot, err := h.svc.ContestMonitor(c.ID, presence)
	if err == nil && snapshot != nil {
		summary.Online = 0
		summary.Participants = len(snapshot.Participants)
		summary.Conflicts = 0
		for _, p := range snapshot.Participants {
			if p.Online {
				summary.Online++
			}
			if p.IPConflict {
				summary.Conflicts++
			}
		}
		summary.Submissions = 0
		for _, p := range snapshot.Participants {
			summary.Submissions += p.SubmissionCount
		}
	}
	stats := h.metrics.RuntimeStatsSnapshot()
	summary.Queued = stats.JudgeJobsQueued

	// 最近提交动态（跨该比赛全部考生，按时间倒序取 8 条）。
	problems, _ := h.svc.Store().ListProblems(c.ID)
	codeOf := map[string]string{}
	for _, p := range problems {
		codeOf[p.ID] = p.Code
	}
	feed := h.svc.ListSubmissionFeed(c.ID, "", 8)
	for _, s := range feed {
		summary.Recent = append(summary.Recent, TraySummaryEvent{
			UserName: s.UserName, ProblemCode: codeOf[s.ProblemID],
			Verdict: s.Verdict, SubmittedAt: s.SubmittedAt,
		})
	}
	return summary
}
