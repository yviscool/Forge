package domain

import "sort"

// ComputeRanking OI 式榜单纯函数：每人每题取最高分，总分降序、满题数降序、姓名升序。
// 纯函数可单测，是榜单地基；并发与存储由上层负责。
func ComputeRanking(users []User, contest Contest, subs []Submission) []RankEntry {
	scores := map[string]*RankEntry{}
	allowed := func(u User) bool { return IsUserAllowed(contest, u) }
	for _, u := range users {
		if allowed(u) {
			scores[u.ID] = &RankEntry{UserID: u.ID, UserName: u.Name, ProblemScores: map[string]int{}}
		}
	}
	for _, x := range subs {
		if x.ContestID != contest.ID {
			continue
		}
		e, ok := scores[x.UserID]
		if !ok {
			e = &RankEntry{UserID: x.UserID, UserName: x.UserName, ProblemScores: map[string]int{}}
			scores[x.UserID] = e
		}
		if x.Score > e.ProblemScores[x.ProblemID] {
			e.ProblemScores[x.ProblemID] = x.Score
		}
	}
	out := make([]RankEntry, 0, len(scores))
	for _, e := range scores {
		total, accepted := 0, 0
		for _, s := range e.ProblemScores {
			total += s
			if s == 100 {
				accepted++
			}
		}
		e.Score, e.Accepted = total, accepted
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Accepted != out[j].Accepted {
			return out[i].Accepted > out[j].Accepted
		}
		return out[i].UserName < out[j].UserName
	})
	return out
}
