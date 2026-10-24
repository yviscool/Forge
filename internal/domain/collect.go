package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CollectEntry 收题包中的一个文件：一名考生在某道题上最后一次提交的源码。
//
// CSP 复赛的收题约定是 <准考证号>/<题面英文名>/<英文名>.cpp，因此目录名
// 来自准考证（无号时退回登录名），文件名来自题面英文名。
type CollectEntry struct {
	UserID      string    `json:"userId"`
	UserName    string    `json:"userName"`
	Username    string    `json:"username"`
	Ticket      string    `json:"ticket,omitempty"`
	Folder      string    `json:"folder"`
	ProblemID   string    `json:"problemId"`
	ProblemCode string    `json:"problemCode"`
	Slug        string    `json:"slug"`
	FileName    string    `json:"fileName"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Verdict     string    `json:"verdict"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submittedAt"`
}

// CollectPackage 一次收题的完整结果：条目按目录名+文件名排序，可稳定落盘；
// Warnings 记录不影响收题但教师应当知道的事（缺号、缺题面英文名、有人交白卷）。
type CollectPackage struct {
	ContestID string         `json:"contestId"`
	Contest   string         `json:"contestName"`
	Season    string         `json:"season,omitempty"`
	Group     string         `json:"group,omitempty"`
	Entries   []CollectEntry `json:"entries"`
	Warnings  []string       `json:"warnings,omitempty"`
}

// CollectSource 收题布局计算的全部输入。domain 不触库：调用方把 store 快照
// 原样递进来，输出完全确定，便于测试与复用（HTTP 导出、CLI 打包）。
type CollectSource struct {
	Contest     Contest
	Groups      []Group
	Users       []User
	Problems    []Problem
	Tickets     []Ticket
	Submissions []Submission
}

// collectLanguageExtension 把语言映射为源文件扩展名。判题支持的语言与
// SupportedLanguage 对齐；未知语言一律落到 .txt，宁可让教师看到原始文件
// 也不能静默丢卷。
func collectLanguageExtension(lang string) string {
	switch NormalizeLanguage(lang) {
	case "cpp":
		return ".cpp"
	case "c":
		return ".c"
	case "python":
		return ".py"
	case "go":
		return ".go"
	default:
		return ".txt"
	}
}

// SanitizeCollectPath 字符串会直接成为 zip 内的目录/文件名：把路径分隔符
// 与 Windows 保留字符替换成下划线，保证一段名字在 zip 里就是一个普通层级。
func SanitizeCollectPath(s string) string {
	repl := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return strings.TrimSpace(repl.Replace(s))
}

// CollectRoster 比赛的收题名单：个人名单与分组名单的并集，按登录名排序。
func CollectRoster(c Contest, groups []Group, users []User) []User {
	byID := make(map[string]User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	seen := map[string]bool{}
	roster := make([]User, 0, len(c.ParticipantUserIDs))
	add := func(uid string) bool {
		if uid == "" || seen[uid] {
			return false
		}
		u, ok := byID[uid]
		if !ok {
			return false
		}
		seen[uid] = true
		roster = append(roster, u)
		return true
	}
	for _, uid := range c.ParticipantUserIDs {
		add(uid)
	}
	for _, gid := range c.GroupIDs {
		for _, g := range groups {
			if g.ID != gid {
				continue
			}
			for _, uid := range g.UserIDs {
				add(uid)
			}
		}
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].Username < roster[j].Username })
	return roster
}

// ComputeCollectLayout 由比赛快照推导收题包。规则：
//
//   - 名单 = 个人绑定 ∪ 分组绑定（CollectRoster）。
//   - 目录名：比赛声明了 ticketGroup 时按（赛季, 组别）反查准考证，找到则
//     用准考证号；没声明组别或该考生无号时退回登录名，并记入 Warnings。
//   - 每人每题只收最后一次提交（与 CSP 收卷口径一致）。
//   - 文件名 = 题面英文名 + 语言扩展名；题面没填英文名时退回题号小写。
func ComputeCollectLayout(src CollectSource) CollectPackage {
	c := src.Contest
	pkg := CollectPackage{
		ContestID: c.ID,
		Contest:   c.Name,
		Season:    c.Season,
		Group:     c.TicketGroup,
		Entries:   []CollectEntry{},
	}

	// 题目按卷面顺序（Contest.ProblemIDs），与考生看到的一致。
	byProblemID := make(map[string]Problem, len(src.Problems))
	for _, p := range src.Problems {
		byProblemID[p.ID] = p
	}
	ordered := make([]Problem, 0, len(c.ProblemIDs))
	for _, pid := range c.ProblemIDs {
		if p, ok := byProblemID[pid]; ok {
			ordered = append(ordered, p)
		}
	}

	// 每题实际使用的英文名：slug 优先，退回题号小写；缺英文名记一条警告。
	slugOf := make(map[string]string, len(ordered))
	for _, p := range ordered {
		slug := NormalizeSlug(p.Slug)
		if slug == "" {
			slug = strings.ToLower(SanitizeCollectPath(p.Code))
			pkg.Warnings = append(pkg.Warnings,
				fmt.Sprintf("problem %s has no slug; using %q as the source file name", p.Code, slug))
		}
		slugOf[p.ID] = slug
	}

	// 准考证查找表：只在本场声明的（赛季, 组别）里找号。
	ticketMode := strings.TrimSpace(c.TicketGroup) != ""
	tickets := make(map[string]Ticket, len(src.Tickets))
	if ticketMode {
		for _, t := range src.Tickets {
			if t.UserID == "" {
				continue
			}
			if _, dup := tickets[t.UserID]; dup {
				continue
			}
			if t.BelongsTo(c.Season, c.TicketGroup) {
				tickets[t.UserID] = t
			}
		}
	}

	// 提交按时间降序扫描：每人每题第一个遇到的就是最后一次提交
	// （与 CSP 收卷口径一致）；时间并列时按提交 ID 决出先后，口径与
	// NOI 榜单计分一致（ranking.go）。
	subs := append([]Submission(nil), src.Submissions...)
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].SubmittedAt.Equal(subs[j].SubmittedAt) {
			return subs[i].ID > subs[j].ID
		}
		return subs[i].SubmittedAt.After(subs[j].SubmittedAt)
	})
	latest := make(map[string]Submission, len(subs))
	order := make([]string, 0, len(subs))
	for _, sub := range subs {
		key := sub.UserID + "|" + sub.ProblemID
		if _, ok := latest[key]; !ok {
			order = append(order, key)
			latest[key] = sub
		}
	}

	roster := CollectRoster(c, src.Groups, src.Users)
	byUser := make(map[string]User, len(roster))
	for _, u := range roster {
		byUser[u.ID] = u
	}

	folderOf := make(map[string]string, len(roster))
	ticketOf := make(map[string]string, len(roster))
	missing := map[string]bool{}
	for _, u := range roster {
		folder := SanitizeCollectPath(u.Username)
		if ticketMode {
			if t, ok := tickets[u.ID]; ok {
				ticketOf[u.ID] = t.Number
				folder = SanitizeCollectPath(t.Number)
			} else if !missing[u.ID] {
				missing[u.ID] = true
				pkg.Warnings = append(pkg.Warnings,
					fmt.Sprintf("participant %s has no %s ticket for season %s; collected under username %s",
						u.Username, c.TicketGroup, c.Season, u.Username))
			}
		}
		folderOf[u.ID] = folder
	}

	for _, key := range order {
		sub, ok := latest[key]
		if !ok {
			continue
		}
		u, okU := byUser[sub.UserID]
		if !okU {
			continue // 提交者不在本场名单里（已移出比赛的历史提交）
		}
		p, okP := byProblemID[sub.ProblemID]
		if !okP {
			continue
		}
		slug := slugOf[sub.ProblemID]
		lang := NormalizeLanguage(sub.Language)
		pkg.Entries = append(pkg.Entries, CollectEntry{
			UserID:      u.ID,
			UserName:    u.Name,
			Username:    u.Username,
			Ticket:      ticketOf[u.ID],
			Folder:      folderOf[u.ID],
			ProblemID:   p.ID,
			ProblemCode: p.Code,
			Slug:        slug,
			FileName:    slug + collectLanguageExtension(lang),
			Language:    lang,
			Code:        sub.Code,
			Verdict:     sub.Verdict,
			Score:       sub.Score,
			SubmittedAt: sub.SubmittedAt,
		})
	}

	// 交白卷提醒：报名了却一题未交，是收题现场最需要当场发现的事。
	submitted := map[string]bool{}
	for _, e := range pkg.Entries {
		submitted[e.UserID] = true
	}
	for _, u := range roster {
		if !submitted[u.ID] {
			pkg.Warnings = append(pkg.Warnings,
				fmt.Sprintf("participant %s submitted nothing", u.Username))
		}
	}

	sort.SliceStable(pkg.Entries, func(i, j int) bool {
		if pkg.Entries[i].Folder != pkg.Entries[j].Folder {
			return pkg.Entries[i].Folder < pkg.Entries[j].Folder
		}
		return pkg.Entries[i].FileName < pkg.Entries[j].FileName
	})
	return pkg
}
