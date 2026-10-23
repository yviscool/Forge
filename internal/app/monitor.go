package app

import (
	"sort"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// MonitorProblemStat 单题监考聚合：尝试次数与最近一次提交形态。
type MonitorProblemStat struct {
	ProblemID    string     `json:"problemId"`
	Code         string     `json:"code"`
	Title        string     `json:"title"`
	Attempts     int        `json:"attempts"`
	LastVerdict  string     `json:"lastVerdict"`
	LastScore    int        `json:"lastScore"`
	LastLanguage string     `json:"lastLanguage"`
	LastAt       *time.Time `json:"lastAt,omitempty"`
}

// MonitorParticipant 监考大屏的选手卡片数据。
type MonitorParticipant struct {
	UserID     string `json:"userId"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	ExamTicket string `json:"examTicket,omitempty"`
	Province   string `json:"province,omitempty"`

	// IP 机位绑定与冲突告警：同一物理 IP 出现在多个账号上即是异常。
	IP         string `json:"ip,omitempty"`
	IPConflict bool   `json:"ipConflict,omitempty"`

	// Online 由 SSE 在线注册表推导（连接存活即在线）。
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`

	ExtensionMinutes int                  `json:"extensionMinutes,omitempty"`
	WindowEndsAt     *time.Time           `json:"windowEndsAt,omitempty"`
	SubmissionCount  int                  `json:"submissionCount"`
	Problems         []MonitorProblemStat `json:"problems"`

	// LastVersion 选手正在编辑的题目（最新代码版本），SSE code.saved 事件推送更新。
	LastVersionProblemID string     `json:"lastVersionProblemId,omitempty"`
	LastVersionSeq       int        `json:"lastVersionSeq,omitempty"`
	LastVersionSource    string     `json:"lastVersionSource,omitempty"`
	LastVersionLines     int        `json:"lastVersionLines,omitempty"`
	LastVersionAt        *time.Time `json:"lastVersionAt,omitempty"`
	LastActivityAt       *time.Time `json:"lastActivityAt,omitempty"`
}

// MonitorSnapshot 监考大屏的整场快照。
type MonitorSnapshot struct {
	ContestID    string               `json:"contestId"`
	Name         string               `json:"name"`
	Mode         string               `json:"mode"`
	Status       string               `json:"status"`
	GeneratedAt  time.Time            `json:"generatedAt"`
	Participants []MonitorParticipant `json:"participants"`
}

// ContestMonitor 汇总一场比赛的全员监考快照。
// presence 是 SSE 在线注册表注入（userID → 连接建立时间），由 transport 层提供。
// 教师专用：学生看不到彼此的任何监考数据。
func (s *Service) ContestMonitor(cid string, presence map[string]time.Time) (*MonitorSnapshot, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return nil, err
	}
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, err
	}
	problems, err := s.store.ListProblems(cid)
	if err != nil {
		return nil, err
	}
	subs, err := s.store.ListSubmissions(cid)
	if err != nil {
		return nil, err
	}

	byID := map[string]domain.User{}
	for _, u := range users {
		byID[u.ID] = u
	}

	// 参赛名单：个人授权 ∪ 组别成员（只含学生角色）。
	authorized := map[string]bool{}
	for _, uid := range c.ParticipantUserIDs {
		authorized[uid] = true
	}
	for _, gid := range c.GroupIDs {
		if g, err := s.store.GetGroup(gid); err == nil {
			for _, uid := range g.UserIDs {
				authorized[uid] = true
			}
		}
	}
	for uid := range byID {
		if byID[uid].Role == domain.RoleStudent && domain.IsUserAllowed(c, byID[uid]) {
			authorized[uid] = true
		}
	}

	// 同 IP 多账号冲突表。
	ipCount := map[string]int{}
	for _, ip := range c.BoundIPs {
		if ip != "" {
			ipCount[ip]++
		}
	}

	// 提交聚合：按（用户,题目）。
	type agg struct {
		attempts int
		last     *domain.Submission
	}
	aggs := map[string]*agg{}
	var lastActivity time.Time
	for i := range subs {
		x := &subs[i]
		key := x.UserID + "\x00" + x.ProblemID
		a := aggs[key]
		if a == nil {
			a = &agg{}
			aggs[key] = a
		}
		a.attempts++
		if a.last == nil || x.SubmittedAt.After(a.last.SubmittedAt) {
			a.last = x
		}
		if x.SubmittedAt.After(lastActivity) {
			lastActivity = x.SubmittedAt
		}
	}

	problemMeta := map[string]domain.Problem{}
	for _, p := range problems {
		problemMeta[p.ID] = p
	}

	// 最新代码版本（正在编辑什么）。
	latestVersion := map[string]domain.CodeVersion{}
	for uid := range authorized {
		for _, p := range problems {
			vs, err := s.store.ListCodeVersions(cid, p.ID, uid, 1)
			if err != nil || len(vs) == 0 {
				continue
			}
			if cur, ok := latestVersion[uid]; !ok || vs[0].CreatedAt.After(cur.CreatedAt) {
				latestVersion[uid] = vs[0]
			}
		}
	}

	now := s.clock.Now()
	out := &MonitorSnapshot{
		ContestID:   c.ID,
		Name:        c.Name,
		Mode:        domain.NormalizeMode(c.Mode),
		Status:      c.Status,
		GeneratedAt: now,
		// 无授权选手时也必须是空数组（JSON null 会击穿前端 map/length）。
		Participants: []MonitorParticipant{},
	}
	for uid := range authorized {
		u := byID[uid]
		p := MonitorParticipant{
			UserID:           uid,
			Name:             u.Name,
			Username:         u.Username,
			ExamTicket:       u.ExamTicket,
			Province:         u.Province,
			ExtensionMinutes: c.Extensions[uid],
			// 无题比赛也要给空数组：JSON null 会击穿前端 map/length。
			Problems: []MonitorProblemStat{},
		}
		if ip := c.BoundIPs[uid]; ip != "" {
			p.IP = ip
			p.IPConflict = ipCount[ip] > 1
		}
		if since, ok := presence[uid]; ok {
			p.Online = true
			if p.LastSeenAt == nil || since.After(*p.LastSeenAt) {
				t := since
				p.LastSeenAt = &t
			}
		}
		if u.CreatedAt.After(time.Time{}) {
			// 注册时间不算活动；无活动时留空，前端显示「未活动」。
			_ = u.CreatedAt
		}

		for _, prob := range problems {
			a := aggs[uid+"\x00"+prob.ID]
			stat := MonitorProblemStat{
				ProblemID: prob.ID, Code: prob.Code, Title: prob.Title,
			}
			if a != nil && a.last != nil {
				stat.Attempts = a.attempts
				stat.LastVerdict = a.last.Verdict
				stat.LastScore = a.last.Score
				stat.LastLanguage = a.last.Language
				lastAt := a.last.SubmittedAt
				stat.LastAt = &lastAt
				p.SubmissionCount += a.attempts
				if p.LastActivityAt == nil || lastAt.After(*p.LastActivityAt) {
					t := lastAt
					p.LastActivityAt = &t
				}
			}
			p.Problems = append(p.Problems, stat)
		}

		if v, ok := latestVersion[uid]; ok {
			p.LastVersionProblemID = v.ProblemID
			p.LastVersionSeq = v.Seq
			p.LastVersionSource = v.Source
			p.LastVersionLines = countLines(v.Code)
			t := v.CreatedAt
			p.LastVersionAt = &t
			if p.LastSeenAt == nil || t.After(*p.LastSeenAt) {
				t2 := t
				p.LastSeenAt = &t2
			}
		}

		// 每人窗口截止 = 结束时间 + 个人加时。
		if c.WindowEnabled && c.EndTime != nil {
			ends := c.EndTime.Add(time.Duration(p.ExtensionMinutes) * time.Minute)
			p.WindowEndsAt = &ends
		}

		out.Participants = append(out.Participants, p)
	}
	sort.Slice(out.Participants, func(i, j int) bool {
		a, b := out.Participants[i], out.Participants[j]
		if a.ExamTicket != b.ExamTicket && a.ExamTicket != "" && b.ExamTicket != "" {
			return a.ExamTicket < b.ExamTicket
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.UserID < b.UserID
	})
	return out, nil
}

func countLines(code string) int {
	if code == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(code); i++ {
		if code[i] == '\n' {
			n++
		}
	}
	return n
}
