package domain

import "testing"

// 部分分聚合（对标 LemonLime：子任务取组内最小分，依赖未过清零）。
func TestAggregateScorePartial(t *testing.T) {
	subtaskOf := func(i int) int { return 1 }
	fullOf := func(st int) int { return 100 }

	// 旧语义回归：WA（无分）→ 0。
	got := AggregateScore([]CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseWA},
	}, subtaskOf, fullOf)
	if got != 0 {
		t.Errorf("WA 无分应得 0，got %d", got)
	}

	// 部分分：AC + Partly(30) → min(100, 30) = 30。
	got = AggregateScore([]CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CasePartly, Score: 30},
	}, subtaskOf, fullOf)
	if got != 30 {
		t.Errorf("部分分应得 30，got %d", got)
	}

	// 全 AC → 满分。
	got = AggregateScore([]CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseAC},
	}, subtaskOf, fullOf)
	if got != 100 {
		t.Errorf("全 AC 应得满分，got %d", got)
	}
}

func TestAggregateScorePartialWithDeps(t *testing.T) {
	// 子任务 1（满分 60，依赖 2）、子任务 2（满分 40）。
	subtaskOf := func(i int) int {
		if i < 2 {
			return 1
		}
		return 2
	}
	fullOf := func(st int) int {
		if st == 1 {
			return 60
		}
		return 40
	}
	depsOf := func(st int) []int {
		if st == 1 {
			return []int{2}
		}
		return nil
	}

	// 依赖子任务部分分（30/40）：子任务 2 未全 AC → 有依赖的子任务 1 清零，
	// 子任务 2 自身保留部分分 30。
	got := AggregateScoreWithDeps([]CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseAC},
		{CaseIndex: 2, Verdict: CasePartly, Score: 30},
	}, subtaskOf, fullOf, depsOf)
	if got != 30 {
		t.Errorf("依赖未过应只保留无依赖子任务的部分分 30，got %d", got)
	}

	// 依赖全过：子任务 2 全 AC（40）+ 子任务 1 全 AC（60）= 100。
	got = AggregateScoreWithDeps([]CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseAC},
		{CaseIndex: 2, Verdict: CaseAC},
	}, subtaskOf, fullOf, depsOf)
	if got != 100 {
		t.Errorf("全 AC 应得 100，got %d", got)
	}
}
