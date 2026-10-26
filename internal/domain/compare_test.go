package domain

import "testing"

// CompareVerdict 的 PE/OLE/WA 三态语义对标 LemonLime compareIgnoreSpaces：
// token 相同 + 行号分叉 → PE；选手多尾部 token → OLE；少尾部/中间不同 → WA。
func TestCompareVerdictIgnoreSpace(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		wantText string
		want     CaseVerdict
	}{
		{"identical", "1 2\n3\n", "1 2\n3\n", CaseAC},
		{"trailing whitespace ignored", "1 2  \n3\t\n", "1 2\n3", CaseAC},
		{"same-line space style differs", "1\t2", "1 2", CaseAC},
		{"token split across lines", "1 2\n3", "1 2 3", CasePE},
		{"blank line inserted", "1\n\n2", "1 2", CasePE},
		{"reordered tokens", "2 1", "1 2", CaseWA},
		{"fewer tokens", "1", "1 2", CaseWA},
		{"empty got", "", "1 2", CaseWA},
		{"both empty", "", "   \n", CaseAC},
	}
	for _, tc := range cases {
		if got := CompareVerdict(tc.got, tc.wantText, CompareIgnoreSpace, 0); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

// 选手多出尾部 token → OLE（对标 upstream compareIgnoreSpaces 的
// "too much contents" 分支）。
func TestCompareVerdictIgnoreSpaceOLE(t *testing.T) {
	if got := CompareVerdict("1 2 3 4", "1 2", CompareIgnoreSpace, 0); got != CaseOLE {
		t.Fatalf("extra trailing tokens must be OLE, got %s", got)
	}
	if got := CompareVerdict("1 2 3", "1 x", CompareIgnoreSpace, 0); got != CaseWA {
		t.Fatalf("mid divergence must be WA, got %s", got)
	}
}

// 其余模式保持两态：line_by_line / real_number / 特判占位。
func TestCompareVerdictOtherModes(t *testing.T) {
	if got := CompareVerdict("1 2\n3", "1 2 3", CompareLineByLine, 0); got != CaseWA {
		t.Fatalf("line_by_line is strict, got %s", got)
	}
	if got := CompareVerdict("1.0001", "1.0002", CompareRealNumber, 1e-3); got != CaseAC {
		t.Fatalf("real_number eps: got %s", got)
	}
	if got := CompareVerdict("a", "a", CompareSpecialJudge, 0); got != CaseWA {
		// 特判占位永不误判 AC（外部 checker 决定结论）。
		t.Fatalf("special judge placeholder: got %s", got)
	}
	// external_tool 在纯 domain 层回退为精确文本相等（真实执行在 judge 层
	// 的 compareExternal）；特判占位永不误判 AC。
	if got := CompareVerdict("same", "same", CompareExternalTool, 0); got != CaseAC {
		t.Fatalf("external tool domain fallback is text equality: got %s", got)
	}
}

// CompareOutput 布尔包装与 CompareVerdict 的一致性。
func TestCompareOutputWrapper(t *testing.T) {
	if !CompareOutput("1 2 3", "1 2 3", CompareIgnoreSpace, 0) {
		t.Fatal("equal must be true")
	}
	if CompareOutput("1 2\n3", "1 2 3", CompareIgnoreSpace, 0) {
		t.Fatal("PE must not count as output-equal")
	}
}

func TestNormalizeComparisonModeExternalTool(t *testing.T) {
	for _, in := range []string{"external_tool", "diff", "diff_tool", " External_Tool "} {
		if got := NormalizeComparisonMode(ComparisonMode(in)); got != CompareExternalTool {
			t.Fatalf("normalize(%q) = %s", in, got)
		}
	}
}
