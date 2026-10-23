package app

import (
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
)

func newVersionTestService(t *testing.T) *Service {
	t.Helper()
	svc := NewService(memory.New(), nil, nil, nil)
	if _, err := svc.CreateContest("版本测试", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProblem(domain.Problem{
		ContestID: "cnt-0001", Code: "A", Title: "两数之和",
		Statement: "求和。", Input: "a b", Output: "s",
		Constraints: "n <= 1e9", Examples: "1 2\n3",
	}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestSaveCodeVersionSequenceAndDedup(t *testing.T) {
	svc := newVersionTestService(t)
	uid := "u1"

	v1, err := svc.SaveCodeVersion("cnt-0001", "", uid, "cpp", "int main(){}", "")
	if err == nil {
		t.Fatal("unknown problem must fail")
	}
	problems := svc.ListProblems("cnt-0001")
	if len(problems) == 0 {
		t.Fatal("problem missing")
	}
	pid := problems[0].ID

	v1, err = svc.SaveCodeVersion("cnt-0001", pid, uid, "cpp", "int main(){}", "")
	if err != nil || v1.Seq != 1 {
		t.Fatalf("v1 = %+v err=%v", v1, err)
	}
	v2, err := svc.SaveCodeVersion("cnt-0001", pid, uid, "cpp", "int main(){return 0;}", "save")
	if err != nil || v2.Seq != 2 {
		t.Fatalf("v2 = %+v err=%v", v2, err)
	}
	// 内容与最近版本一致 → 幂等复用，不追加。
	v2again, err := svc.SaveCodeVersion("cnt-0001", pid, uid, "cpp", "int main(){return 0;}", "save")
	if err != nil || v2again.Seq != 2 || v2again.ID != v2.ID {
		t.Fatalf("dedup = %+v err=%v", v2again, err)
	}
	// 换语言即使内容同也算新版本。
	v3, err := svc.SaveCodeVersion("cnt-0001", pid, uid, "python", "int main(){return 0;}", "")
	if err != nil || v3.Seq != 3 {
		t.Fatalf("v3 = %+v err=%v", v3, err)
	}
	// 非法来源。
	if _, err := svc.SaveCodeVersion("cnt-0001", pid, uid, "cpp", "x", "autosync"); err == nil {
		t.Fatal("invalid source must fail")
	}
	// 超体积拒绝。
	if _, err := svc.SaveCodeVersion("cnt-0001", pid, uid, "cpp", strings.Repeat("a", domain.MaxCodeVersionBytes+1), ""); err == nil {
		t.Fatal("oversized version must fail")
	}

	// 学生只能看自己的；教师任意。
	if _, err := svc.ListCodeVersions("student", "u2", "cnt-0001", pid, "u1", 0); err == nil {
		t.Fatal("student reading others must fail")
	}
	own, err := svc.ListCodeVersions("student", uid, "cnt-0001", pid, uid, 0)
	if err != nil || len(own) != 3 || own[0].Seq != 3 {
		t.Fatalf("own list = %+v err=%v", own, err)
	}
	staff, err := svc.ListCodeVersions("teacher", "t1", "cnt-0001", pid, uid, 0)
	if err != nil || len(staff) != 3 {
		t.Fatalf("staff list = %+v err=%v", staff, err)
	}
}

func TestSaveCodeVersionFinishedContestReadOnly(t *testing.T) {
	svc := newVersionTestService(t)
	if _, err := svc.CreateContest("已结束", ""); err != nil {
		t.Fatal(err)
	}
	// 把第二场比赛推进到 finished。
	contests, err := svc.Store().ListContests()
	if err != nil {
		t.Fatal(err)
	}
	var finishedID string
	for _, c := range contests {
		if c.ID != "cnt-0001" {
			finishedID = c.ID
		}
	}
	if finishedID == "" {
		t.Fatal("second contest missing")
	}
	if _, err := svc.CreateProblem(domain.Problem{
		ContestID: finishedID, Code: "A", Title: "结束题",
		Statement: "求积。", Input: "a b", Output: "s",
		Constraints: "n <= 1e9", Examples: "2 3\n6",
	}); err != nil {
		t.Fatal(err)
	}
	// 建题后再冻结成已结束（结束后题面只读、版本只读）。
	for i := range contests {
		if contests[i].ID == finishedID {
			contests[i].Status = "finished"
			if err := svc.Store().UpdateContest(contests[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	problems2, err := svc.Store().ListProblems(finishedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems2) == 0 {
		t.Fatal("second contest has no problem")
	}
	if _, err := svc.SaveCodeVersion(finishedID, problems2[0].ID, "u1", "cpp", "x", ""); err == nil {
		t.Fatal("finished contest must reject new versions")
	}
}

func TestSubmitRecordsVersion(t *testing.T) {
	svc := newVersionTestService(t)
	// 学生 + 授权。
	u, err := svc.CreateUser("考生甲", "student")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddParticipantToContest("cnt-0001", u.ID); err != nil {
		t.Fatal(err)
	}
	// 启动比赛（先补齐启动校验所需的学生与题目）。
	if _, err := svc.StartContest("cnt-0001"); err != nil {
		t.Fatal(err)
	}
	problems := svc.ListProblems("cnt-0001")
	pid := problems[0].ID

	sub, err := svc.Submit("cnt-0001", pid, u.ID, "cpp", "int sol(){return 42;}")
	if err != nil {
		t.Fatal(err)
	}
	versions, err := svc.ListCodeVersions("teacher", "t1", "cnt-0001", pid, u.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("submit must record exactly one version, got %d", len(versions))
	}
	v := versions[0]
	if v.Source != domain.VersionSourceSubmit || v.Code != sub.Code || v.Seq != 1 {
		t.Fatalf("submit version = %+v", v)
	}

	// 内容相同的提交不重复占号；内容不同则新版本。
	if _, err := svc.Submit("cnt-0001", pid, u.ID, "cpp", "int sol(){return 42;}"); err != nil {
		t.Fatal(err)
	}
	versions, _ = svc.ListCodeVersions("teacher", "t1", "cnt-0001", pid, u.ID, 0)
	if len(versions) != 1 {
		t.Fatalf("identical resubmit must not add version, got %d", len(versions))
	}
	if _, err := svc.Submit("cnt-0001", pid, u.ID, "cpp", "int sol(){return 43;}"); err != nil {
		t.Fatal(err)
	}
	versions, _ = svc.ListCodeVersions("teacher", "t1", "cnt-0001", pid, u.ID, 0)
	if len(versions) != 2 || versions[0].Code != "int sol(){return 43;}" {
		t.Fatalf("changed resubmit must add version: %+v", versions)
	}
}
