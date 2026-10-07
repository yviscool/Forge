package app

import (
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func TestContestLifecycleAndStateGuards(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)

	u, err := svc.CreateUser("Alice", domain.RoleStudent)
	if err != nil {
		t.Fatal(err)
	}

	c, err := svc.CreateContest("Demo Contest", "Testing lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != string(domain.Draft) {
		t.Fatalf("new contest must be draft, got %s", c.Status)
	}

	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID,
		Code:      "A",
		Title:     "Problem A",
		Statement: "Statement",
		Input:       "Input",
		Output:      "Output",
		Constraints: "N <= 100",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. 在 Draft 态不能直接 Finish
	if _, err := svc.FinishContest(c.ID); err == nil {
		t.Fatal("Draft contest should not be directly finishable")
	}

	// 2. 在 Draft 态不能提交代码
	if _, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "int main(){}"); err == nil {
		t.Fatal("Draft contest must reject submissions")
	}

	// 3. Draft 态允许修改设置和测试点
	if _, err := svc.SetContestLanguages(c.ID, []string{"cpp", "python"}); err != nil {
		t.Fatalf("Draft should allow modifying languages: %v", err)
	}

	// 4. 启动比赛 -> Running
	c, err = svc.StartContest(c.ID)
	if err != nil {
		t.Fatalf("failed to start contest: %v", err)
	}
	if c.Status != string(domain.Running) {
		t.Fatalf("expected running, got %s", c.Status)
	}

	// 5. Running 态不能重复启动
	if _, err := svc.StartContest(c.ID); err == nil {
		t.Fatal("Running contest cannot be started again")
	}

	// 6. Running 态允许提交
	sub, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "int main(){}")
	if err != nil {
		t.Fatalf("Running contest must accept submission: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("submission should have an ID")
	}

	// 7. 结束比赛 -> Finished
	c, err = svc.FinishContest(c.ID)
	if err != nil {
		t.Fatalf("failed to finish contest: %v", err)
	}
	if c.Status != string(domain.Finished) {
		t.Fatalf("expected finished, got %s", c.Status)
	}

	// 8. Finished 态全面冻结防护：
	// 不能再次启动
	if _, err := svc.StartContest(c.ID); err == nil {
		t.Fatal("Finished contest must not restart")
	}
	// 不能再次结束
	if _, err := svc.FinishContest(c.ID); err == nil {
		t.Fatal("Finished contest must not finish again")
	}
	// 不能提交
	if _, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "int main(){}"); err == nil {
		t.Fatal("Finished contest must reject submissions")
	}
	// 不能新增题目
	if _, err := svc.CreateProblem(domain.Problem{
		ContestID:   c.ID,
		Code:        "B",
		Title:       "Problem B",
		Statement:   "Statement",
		Input:       "Input",
		Output:      "Output",
		Constraints: "N <= 100",
	}); err == nil {
		t.Fatal("Finished contest must reject problem creation")
	}
	// 不能修改题目
	if _, err := svc.UpdateProblem(domain.Problem{
		ID:        p.ID,
		ContestID: c.ID,
		Title:     "Modified Title",
	}); err == nil {
		t.Fatal("Finished contest must reject problem modification")
	}
	// 不能修改语言白名单
	if _, err := svc.SetContestLanguages(c.ID, []string{"go"}); err == nil {
		t.Fatal("Finished contest must reject language settings modification")
	}
	// 不能修改榜单模式
	if _, err := svc.SetRankingMode(c.ID, "acm"); err == nil {
		t.Fatal("Finished contest must reject ranking mode modification")
	}
}

func TestImportBundleAtomicity(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)

	// 初始比赛数量
	initialContests := svc.ListContests()

	// 构造一个包含错误题目的 Bundle（第二道题目缺少必要字段，触发校验失败）
	badBundle := ContestBundle{
		Contest: domain.Contest{
			Name: "Broken Contest",
		},
		Problems: []domain.Problem{
			{
				Code:        "A",
				Title:       "Valid A",
				Statement:   "S",
				Input:       "I",
				Output:      "O",
				Constraints: "N <= 100",
			},
			{
				// Missing Title, Statement, Input, Output
				Code: "B",
			},
		},
	}

	_, err := svc.ImportBundle(badBundle)
	if err == nil {
		t.Fatal("importing invalid bundle must return error")
	}

	// 验证事务原子性回滚：比赛未被创建，任何题目都未遗留
	currentContests := svc.ListContests()
	if len(currentContests) != len(initialContests) {
		t.Fatalf("bundle import failure should rollback contest creation, expected %d contests, got %d",
			len(initialContests), len(currentContests))
	}
}
