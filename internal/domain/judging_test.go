package domain

import "testing"

func TestCompareOutput(t *testing.T) {
	if !CompareOutput("1 2\n", "1 2", CompareLineByLine, 0) {
		t.Fatal("trailing newline should be ignored")
	}
	if CompareOutput("1 2", "1 3", CompareLineByLine, 0) {
		t.Fatal("different output must not match")
	}
	if !CompareOutput("1  2\n3", "1 2 3", CompareIgnoreSpace, 0) {
		t.Fatal("ignore-space mode broken")
	}
	if !CompareOutput("0.333", "0.3334", CompareRealNumber, 1e-3) {
		t.Fatal("real-number within eps should pass")
	}
	if CompareOutput("0.33", "0.34", CompareRealNumber, 1e-3) {
		t.Fatal("real-number outside eps must fail")
	}
	if !CompareOutput("hello", "hello", CompareRealNumber, 0) {
		t.Fatal("non-numeric tokens fall back to string compare")
	}
	if CompareOutput("x", "y", CompareSpecialJudge, 0) {
		t.Fatal("special judge placeholder must not auto-AC")
	}
}

func TestNormalizeComparisonMode(t *testing.T) {
	cases := map[ComparisonMode]ComparisonMode{
		"":                       "",
		"exact":                  CompareLineByLine,
		"strict":                 CompareLineByLine,
		"token":                  CompareIgnoreSpace,
		"ignore_whitespace":      CompareIgnoreSpace,
		"float_eps":              CompareRealNumber,
		"real_number":            CompareRealNumber,
		"custom_checker":         CompareSpecialJudge,
		"special_judge":          CompareSpecialJudge,
		"  IGNORE_SPACE  ":       CompareIgnoreSpace,
		ComparisonMode("future"): ComparisonMode("future"),
	}
	for input, want := range cases {
		if got := NormalizeComparisonMode(input); got != want {
			t.Fatalf("normalize %q: got %q, want %q", input, got, want)
		}
	}
}

func TestAggregateScoreSubtaskDep(t *testing.T) {
	cases := []CaseResult{
		{CaseIndex: 0, Verdict: CaseAC},
		{CaseIndex: 1, Verdict: CaseWA},
		{CaseIndex: 2, Verdict: CaseAC},
	}
	subtaskOf := func(i int) int {
		if i < 2 {
			return 0
		}
		return 1
	}
	fullOf := func(st int) int {
		if st == 0 {
			return 40
		}
		return 60
	}
	if got := AggregateScore(cases, subtaskOf, fullOf); got != 60 {
		t.Fatalf("subtask0 has WA so only subtask1 counts, got %d", got)
	}
	allAC := []CaseResult{{CaseIndex: 0, Verdict: CaseAC}, {CaseIndex: 2, Verdict: CaseAC}}
	if got := AggregateScore(allAC, subtaskOf, fullOf); got != 100 {
		t.Fatalf("all AC should be 100, got %d", got)
	}
}
