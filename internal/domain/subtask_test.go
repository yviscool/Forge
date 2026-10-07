package domain

import (
	"testing"
)

func TestSubtaskDAGDependencies(t *testing.T) {
	// 4 个子任务：
	// Subtask 0: 20 分 (测试点 0, 1)
	// Subtask 1: 30 分 (测试点 2, 3), 依赖 Subtask 0
	// Subtask 2: 20 分 (测试点 4), 依赖 Subtask 0
	// Subtask 3: 30 分 (测试点 5), 依赖 Subtask 1 和 2 (菱形依赖)
	fullOf := func(st int) int {
		switch st {
		case 0:
			return 20
		case 1:
			return 30
		case 2:
			return 20
		case 3:
			return 30
		default:
			return 0
		}
	}
	subtaskOf := func(ci int) int {
		switch ci {
		case 0, 1:
			return 0
		case 2, 3:
			return 1
		case 4:
			return 2
		case 5:
			return 3
		default:
			return -1
		}
	}
	depsOf := func(st int) []int {
		switch st {
		case 1, 2:
			return []int{0}
		case 3:
			return []int{1, 2}
		default:
			return nil
		}
	}

	// 场景 1: 全部 AC -> 满分 100
	casesAllAC := []CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseAC},
		{CaseIndex: 2, Verdict: CaseAC},
		{CaseIndex: 3, Verdict: CaseAC},
		{CaseIndex: 4, Verdict: CaseAC},
		{CaseIndex: 5, Verdict: CaseAC},
	}
	if score := AggregateScoreWithDeps(casesAllAC, subtaskOf, fullOf, depsOf); score != 100 {
		t.Fatalf("expected 100, got %d", score)
	}

	// 场景 2: Subtask 2 失败 (Case 4 WA)
	// Subtask 0 AC (20分), Subtask 1 AC (30分), Subtask 2 失败 (0分)
	// Subtask 3 虽然 Case 5 AC，但因依赖 Subtask 2 失败，所以 Subtask 3 得 0分！
	// 总分应为 20 + 30 = 50 分
	casesSub2Failed := []CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseAC},
		{CaseIndex: 2, Verdict: CaseAC},
		{CaseIndex: 3, Verdict: CaseAC},
		{CaseIndex: 4, Verdict: CaseWA},
		{CaseIndex: 5, Verdict: CaseAC},
	}
	if score := AggregateScoreWithDeps(casesSub2Failed, subtaskOf, fullOf, depsOf); score != 50 {
		t.Fatalf("expected 50 (subtask 3 should get 0 due to dep 2 failing), got %d", score)
	}

	// 场景 3: Subtask 0 失败 (Case 1 WA)
	// 所有依赖它的 Subtask 1, 2, 3 全部 0 分！总分 0
	casesSub0Failed := []CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseWA},
		{CaseIndex: 2, Verdict: CaseAC},
		{CaseIndex: 3, Verdict: CaseAC},
		{CaseIndex: 4, Verdict: CaseAC},
		{CaseIndex: 5, Verdict: CaseAC},
	}
	if score := AggregateScoreWithDeps(casesSub0Failed, subtaskOf, fullOf, depsOf); score != 0 {
		t.Fatalf("expected 0, got %d", score)
	}

	// 场景 4: 循环依赖防死递归测试
	cycleDeps := func(st int) []int {
		if st == 0 {
			return []int{1}
		}
		if st == 1 {
			return []int{0}
		}
		return nil
	}
	// 即使全 AC，循环依赖也绝不会导致死循环崩溃，得 0 分
	if score := AggregateScoreWithDeps(casesAllAC[:4], subtaskOf, fullOf, cycleDeps); score != 0 {
		t.Fatalf("cyclic dependency should yield 0, got %d", score)
	}
}

func TestShouldSkipCaseLogic(t *testing.T) {
	failed := map[int]bool{0: true}

	// 测试点所属子任务已失败 -> 跳过
	if !ShouldSkipCase(0, nil, failed) {
		t.Fatal("subtask 0 already failed, case should be skipped")
	}

	// 测试点依赖子任务 0，子任务 0 已失败 -> 跳过
	if !ShouldSkipCase(1, []int{0}, failed) {
		t.Fatal("dependent subtask 0 failed, case should be skipped")
	}

	// 测试点无依赖且自身未失败 -> 不跳过
	if ShouldSkipCase(2, nil, failed) {
		t.Fatal("subtask 2 not failed, should not skip")
	}
}

func TestContestStateMachineTransitions(t *testing.T) {
	// Draft -> Running -> Finished
	if !Draft.CanTransitionTo(Running) {
		t.Fatal("Draft should transition to Running")
	}
	if !Running.CanTransitionTo(Finished) {
		t.Fatal("Running should transition to Finished")
	}

	// 非法转移
	if Draft.CanTransitionTo(Finished) {
		t.Fatal("Draft cannot transition directly to Finished")
	}
	if Finished.CanTransitionTo(Running) {
		t.Fatal("Finished cannot transition to Running")
	}
	if Finished.CanTransitionTo(Draft) {
		t.Fatal("Finished cannot transition to Draft")
	}
	if Running.CanTransitionTo(Draft) {
		t.Fatal("Running cannot transition to Draft")
	}

	// 状态守卫
	cDraft := Contest{Status: string(Draft)}
	if !cDraft.CanModifySettings() {
		t.Fatal("Draft should allow modifying settings")
	}
	if cDraft.CanSubmit() {
		t.Fatal("Draft should not allow submit")
	}

	cRunning := Contest{Status: string(Running)}
	if !cRunning.CanModifySettings() {
		t.Fatal("Running should allow emergency settings adjustment")
	}
	if !cRunning.CanSubmit() {
		t.Fatal("Running should allow submit")
	}

	cFinished := Contest{Status: string(Finished)}
	if cFinished.CanModifySettings() {
		t.Fatal("Finished must freeze settings")
	}
	if cFinished.CanSubmit() {
		t.Fatal("Finished must reject submit")
	}
}
