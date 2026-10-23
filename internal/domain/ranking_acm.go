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
	// A deleted problem stays in historical submissions and therefore must
	// never keep contributing solves or penalty to the live board. When the
	// manifest is absent (legacy fixture/import), retain the old permissive
	// behavior and consider every submission part of the contest.
	validProblems := make(map[string]bool, len(contest.ProblemIDs))
	for _, id := range contest.ProblemIDs {
		validProblems[id] = true
	}
	byUser := map[string][]Submission{}
	for _, s := range subs {
		if s.ContestID != contest.ID {
			continue
		}
		if len(validProblems) > 0 && !validProblems[s.ProblemID] {
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
		// Operational accounts can submit on behalf of a class, but they are
		// not contestants and must never pollute the public ACM board.
		if u, ok := findUser(users, uid); !ok || !isContestant(u) || !IsUserAllowed(contest, u) {
			continue
		}
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
			} else if s.Verdict != VerdictQueued && s.Verdict != VerdictJudging && s.Verdict != VerdictCompileError && s.Verdict != string(CaseCE) && s.Verdict != VerdictSkipped && s.Verdict != string(CaseSkipped) {
				waCnt[s.ProblemID]++
			}
		}
		name := nameOf[uid]
		if name == "" {
			name = uid
		}
		out = append(out, ACMRankEntry{UserID: uid, UserName: name, Solved: solved, Penalty: penalty})
	}
	// 参赛但零提交的用户也上榜（0 题），以稳定排序插入。
	seen := map[string]bool{}
	for _, e := range out {
		seen[e.UserID] = true
	}
	for _, u := range users {
		if isContestant(u) && IsUserAllowed(contest, u) && !seen[u.ID] {
			out = append(out, ACMRankEntry{UserID: u.ID, UserName: u.Name, Solved: 0, Penalty: 0})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Solved != out[j].Solved {
			return out[i].Solved > out[j].Solved
		}
		if out[i].Penalty != out[j].Penalty {
			return out[i].Penalty < out[j].Penalty
		}
		if out[i].UserName != out[j].UserName {
			return out[i].UserName < out[j].UserName
		}
		return out[i].UserID < out[j].UserID
	})
	return out
}

func findUser(users []User, id string) (User, bool) {
	for _, u := range users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

func isContestant(u User) bool {
	return u.Role == RoleStudent || u.Role == ""
}
