package domain

import "sort"

// ACMRankEntry ACM 榜条目：解题数降序、罚时升序、姓名升序。
type ACMRankEntry struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Solved   int    `json:"solved"`
	Penalty  int    `json:"penaltyMin"`
}

// ComputeRankingACM ACM 赛制：每题首个 AC 前的错误提交每次 +20 分钟罚时，
// 罚时按提交时间相对比赛开始的分钟数累加（无开始时间则用提交先后相对值）。
func ComputeRankingACM(users []User, contest Contest, subs []Submission) []ACMRankEntry {
	byUser := map[string][]Submission{}
	for _, s := range subs {
		if s.ContestID != contest.ID {
			continue
		}
		byUser[s.UserID] = append(byUser[s.UserID], s)
	}
	nameOf := map[string]string{}
	for _, u := range users {
		nameOf[u.ID] = u.Name
	}
	out := []ACMRankEntry{}
	for uid, list := range byUser {
		sort.Slice(list, func(i, j int) bool { return list[i].SubmittedAt.Before(list[j].SubmittedAt) })
		solved, penalty := 0, 0
		acAt := map[string]int64{}
		waCnt := map[string]int{}
		base := contest.StartedAt.Unix()
		for _, s := range list {
			if _, ok := acAt[s.ProblemID]; ok {
				continue
			}
			mins := 0
			if base > 0 {
				mins = int((s.SubmittedAt.Unix() - base) / 60)
				if mins < 0 {
					mins = 0
				}
			}
			if s.Verdict == VerdictAccepted || s.Verdict == string(CaseAC) {
				acAt[s.ProblemID] = int64(mins)
				solved++
				penalty += mins + 20*waCnt[s.ProblemID]
			} else if s.Verdict != VerdictQueued && s.Verdict != VerdictJudging {
				waCnt[s.ProblemID]++
			}
		}
		name := nameOf[uid]
		if name == "" {
			name = uid
		}
		out = append(out, ACMRankEntry{UserID: uid, UserName: name, Solved: solved, Penalty: penalty})
	}
	// 参赛但零提交的用户也上榜（0 题）。
	allowed := map[string]bool{}
	for _, u := range users {
		if IsUserAllowed(contest, u) {
			allowed[u.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, e := range out {
		seen[e.UserID] = true
	}
	for uid := range allowed {
		if !seen[uid] {
			out = append(out, ACMRankEntry{UserID: uid, UserName: nameOf[uid], Solved: 0, Penalty: 0})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Solved != out[j].Solved {
			return out[i].Solved > out[j].Solved
		}
		if out[i].Penalty != out[j].Penalty {
			return out[i].Penalty < out[j].Penalty
		}
		return out[i].UserName < out[j].UserName
	})
	return out
}
