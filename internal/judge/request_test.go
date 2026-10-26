package judge

import (
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestBuildRequestModes(t *testing.T) {
	sub := domain.Submission{ID: "s1", Language: "cpp", Code: "ans"}
	// 无测试点：拒绝自动评测。
	if _, ok := BuildRequest(sub, domain.Problem{}); ok {
		t.Fatal("empty testcases must refuse")
	}
	base := domain.Problem{
		Code: "A", TimeLimitMs: 2000, MemoryLimitMiB: 256,
		TestCases: []domain.TestCase{
			{ID: "t1", InputFile: "1", OutputFile: "1", Score: 50, Subtask: 0, TimeLimitMs: 400},
			{ID: "t2", InputFile: "2", OutputFile: "2", Score: 50, Subtask: 1},
		},
	}
	req, ok := BuildRequest(sub, base)
	if !ok {
		t.Fatal("should build")
	}
	if req.Cases[0].Limits.TimeMs != 400 || req.Cases[1].Limits.TimeMs != 0 {
		t.Fatalf("per-case limits: %+v", req.Cases)
	}
	if req.Cases[0].Subtask != 0 || req.Cases[1].Subtask != 1 {
		t.Fatalf("subtask indices must reach the judge: %+v", req.Cases)
	}
	if req.Limits.TimeMs != 2000 || req.Compare != domain.CompareIgnoreSpace {
		t.Fatalf("defaults: %+v", req.Limits)
	}
	// 特判。
	sp := base
	sp.CheckerCode = "int main(){}"
	req, _ = BuildRequest(sub, sp)
	if req.Checker == nil || req.Compare != domain.CompareSpecialJudge {
		t.Fatal("checker wiring")
	}
	// 答案题。
	ap := base
	ap.TaskType = domain.TaskAnswersOnly
	req, _ = BuildRequest(sub, ap)
	if req.TaskType != domain.TaskAnswersOnly || len(req.Cases) != 2 {
		t.Fatal("answers-only wiring")
	}
	// 文件 IO 默认文件名。
	fp := base
	fp.IOMode = domain.IOFile
	req, _ = BuildRequest(sub, fp)
	if req.Limits.InFile != "A.in" || req.Limits.OutFile != "A.out" {
		t.Fatalf("file defaults: %+v", req.Limits)
	}
	// 交互。
	ip := base
	ip.TaskType = domain.TaskInteraction
	ip.InteractorCode = "x"
	req, _ = BuildRequest(sub, ip)
	if req.Interactor == nil {
		t.Fatal("interactor wiring")
	}

	// Dependencies and per-case subtasks drive skip/aggregation semantics.
	dep := base
	dep.TestCases = []domain.TestCase{
		{ID: "d1", Subtask: 1, DependsOn: []int{2}},
		{ID: "d2", Subtask: 2},
	}
	req, _ = BuildRequest(sub, dep)
	if req.Cases[0].Subtask != 1 || len(req.Cases[0].DependsOn) != 1 || req.Cases[0].DependsOn[0] != 2 {
		t.Fatalf("dependency wiring: %+v", req.Cases)
	}
}

func TestBuildRequestProblemOverrides(t *testing.T) {
	sub := domain.Submission{ID: "s1", Language: "cpp", Code: "x"}
	p := domain.Problem{
		Code: "A", TimeLimitMs: 1000, MemoryLimitMiB: 256,
		CompareMode: domain.CompareExternalTool, DiffCommand: "diff",
		DiffArgs:         []string{"-q"},
		CompileStd:       "c++17",
		TimeLimitRatio:   2,
		MemoryLimitRatio: 1.5,
		TestCases:        []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	}
	disable := true
	p.DisableMemoryLimitCheck = &disable
	req, ok := BuildRequest(sub, p)
	if !ok {
		t.Fatal("should build")
	}
	if req.Diff == nil || req.Diff.Command != "diff" || len(req.Diff.Args) != 1 {
		t.Fatalf("external diff wiring: %+v", req.Diff)
	}
	if req.CompileStd != "c++17" || req.TimeLimitRatio != 2 || req.MemoryLimitRatio != 1.5 {
		t.Fatalf("per-problem overrides: %+v", req)
	}
	if req.DisableMemoryLimitCheck == nil || !*req.DisableMemoryLimitCheck {
		t.Fatalf("disable memory limit wiring: %+v", req.DisableMemoryLimitCheck)
	}
}
