package domain

import "sort"

// ComputeRanking OI 式榜单纯函数：每人每题取最高分，总分降序、满题数降序、姓名升序、用户ID升序。
// fullScores 可选传题目满分映射（默认满分 100）。
func ComputeRanking(users []User, contest Contest, subs []Submission, fullScores ...map[string]int) []RankEntry {
	scores := map[string]*RankEntry{}
	allowedMap := map[string]bool{}
	for _, u := range users {
		if IsUserAllowed(contest, u) {
			allowedMap[u.ID] = true
			scores[u.ID] = &RankEntry{UserID: u.ID, UserName: u.Name, ProblemScores: map[string]int{}}
		}
	}

	for _, x := range subs {
		if x.ContestID != contest.ID {
			continue
		}
		if !allowedMap[x.UserID] {
			continue
		}
		e := scores[x.UserID]
		if x.Score > e.ProblemScores[x.ProblemID] {
			e.ProblemScores[x.ProblemID] = x.Score
		}
	}

	var maxScoreMap map[string]int
	if len(fullScores) > 0 {
		maxScoreMap = fullScores[0]
	}

	out := make([]RankEntry, 0, len(scores))
	for _, e := range scores {
		total, accepted := 0, 0
		for pid, s := range e.ProblemScores {
			total += s
			full := 100
			if maxScoreMap != nil && maxScoreMap[pid] > 0 {
				full = maxScoreMap[pid]
			}
			if s >= full {
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
		if out[i].UserName != out[j].UserName {
			return out[i].UserName < out[j].UserName
		}
		return out[i].UserID < out[j].UserID
	})
	return out
}
