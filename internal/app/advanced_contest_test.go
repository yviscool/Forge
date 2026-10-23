package app_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
)

type fixedClock struct {
	now time.Time
}

func (f *fixedClock) Now() time.Time          { return f.now }
func (f *fixedClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

func setupTestService(t *testing.T) (*app.Service, *fixedClock) {
	st := memory.New()
	clk := &fixedClock{now: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)}
	svc := app.NewService(st, nil, clk, nil)
	return svc, clk
}

func TestIPBindingAndMismatchRejection(t *testing.T) {
	svc, _ := setupTestService(t)
	c, _ := svc.CreateContest("CSP-J Final", "", "oi")
	u1, _ := svc.CreateUser("Alice", "student")
	u2, _ := svc.CreateUser("Bob", "student")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "Prob A", Statement: "stmt",
		Input: "in", Output: "out", Constraints: "none",
	})
	_ = svc.AddParticipantToContest(c.ID, u1.ID)
	_ = svc.AddParticipantToContest(c.ID, u2.ID)
	_, _ = svc.StartContest(c.ID)

	// 开启 IP 绑定
	_, err := svc.SetIPBindingEnabled(c.ID, true)
	if err != nil {
		t.Fatalf("SetIPBindingEnabled failed: %v", err)
	}

	// u1 首次从 192.168.1.101 提交代码 -> 自动锁定机位 IP
	sub1, err := svc.SubmitWithClient(c.ID, p.ID, u1.ID, "cpp", "code1", "192.168.1.101")
	if err != nil {
		t.Fatalf("First submission failed: %v", err)
	}
	if sub1.ID == "" {
		t.Fatalf("expected valid submission id")
	}

	// 验证绑定表
	bindings, enabled, err := svc.GetBoundIPs(c.ID)
	if err != nil || !enabled {
		t.Fatalf("expected enabled, got err: %v", err)
	}
	if bindings[u1.ID] != "192.168.1.101" {
		t.Fatalf("expected bound IP 192.168.1.101, got %s", bindings[u1.ID])
	}

	// u1 尝试从异地 192.168.1.200 提交 -> 必须被硬阻断！
	_, err = svc.SubmitWithClient(c.ID, p.ID, u1.ID, "cpp", "code2", "192.168.1.200")
	if err == nil || !strings.Contains(err.Error(), "ip_mismatch") {
		t.Fatalf("expected ip_mismatch error, got: %v", err)
	}

	// 教师为 u1 一键解绑（如死机换机）
	if err := svc.UnbindUserIP(c.ID, u1.ID); err != nil {
		t.Fatalf("UnbindUserIP failed: %v", err)
	}

	// u1 换到新机位 192.168.1.105 重新提交 -> 自动重新绑定新机位
	_, err = svc.SubmitWithClient(c.ID, p.ID, u1.ID, "cpp", "code3", "192.168.1.105")
	if err != nil {
		t.Fatalf("Submission after unbind failed: %v", err)
	}

	bindings, _, _ = svc.GetBoundIPs(c.ID)
	if bindings[u1.ID] != "192.168.1.105" {
		t.Fatalf("expected new bound IP 192.168.1.105, got %s", bindings[u1.ID])
	}

	// 全员一键解绑测试
	if err := svc.UnbindAllIPs(c.ID); err != nil {
		t.Fatalf("UnbindAllIPs failed: %v", err)
	}
	bindings, _, _ = svc.GetBoundIPs(c.ID)
	if len(bindings) != 0 {
		t.Fatalf("expected empty bindings after unbind all, got %v", bindings)
	}
}

func TestContestWindowAndAccurateExtension(t *testing.T) {
	svc, clk := setupTestService(t)
	c, _ := svc.CreateContest("GESP Level 4 Exam", "", "oi")
	u1, _ := svc.CreateUser("Charlie", "student")
	u2, _ := svc.CreateUser("David", "student")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "P1", Title: "Problem 1", Statement: "stmt",
		Input: "in", Output: "out", Constraints: "none",
	})
	_ = svc.AddParticipantToContest(c.ID, u1.ID)
	_ = svc.AddParticipantToContest(c.ID, u2.ID)
	_, _ = svc.StartContest(c.ID)

	// 设定考试窗口：9:00 - 11:30
	startTime := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 10, 8, 11, 30, 0, 0, time.UTC)
	_, err := svc.SetContestWindow(c.ID, true, &startTime, &endTime, 150)
	if err != nil {
		t.Fatalf("SetContestWindow failed: %v", err)
	}

	// 当前时钟: 8:50 (未开考) -> 提交被拒
	clk.now = time.Date(2026, 10, 8, 8, 50, 0, 0, time.UTC)
	_, err = svc.Submit(c.ID, p.ID, u1.ID, "cpp", "code")
	if err == nil || !strings.Contains(err.Error(), "exam has not started") {
		t.Fatalf("expected not started error, got %v", err)
	}

	// 当前时钟: 10:00 (考试中) -> 正常提交
	clk.now = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	sub, err := svc.Submit(c.ID, p.ID, u1.ID, "cpp", "code")
	if err != nil {
		t.Fatalf("Submit during window failed: %v", err)
	}
	if sub.ID == "" {
		t.Fatalf("expected valid submission")
	}

	// 当前时钟: 11:35 (考试已截止)
	clk.now = time.Date(2026, 10, 8, 11, 35, 0, 0, time.UTC)
	// u1 没有加时 -> 硬切断拦截！
	_, err = svc.Submit(c.ID, p.ID, u1.ID, "cpp", "code")
	if err == nil || !strings.Contains(err.Error(), "exam time has ended") {
		t.Fatalf("expected expired error for u1, got %v", err)
	}

	// 教师为死机的 u2 紧急加时 15 分钟（截止时间顺延至 11:45）
	_, err = svc.SetUserExtension(c.ID, u2.ID, 15)
	if err != nil {
		t.Fatalf("SetUserExtension failed: %v", err)
	}

	// 11:35 分，u2 拥有加时 -> 成功提交！
	sub2, err := svc.Submit(c.ID, p.ID, u2.ID, "cpp", "code u2")
	if err != nil {
		t.Fatalf("Submit with extension should succeed, got: %v", err)
	}
	if sub2.ID == "" {
		t.Fatalf("expected valid submission for u2")
	}

	// 时钟推进到 11:46 (超出 u2 加时) -> u2 同样被硬切断！
	clk.now = time.Date(2026, 10, 8, 11, 46, 0, 0, time.UTC)
	_, err = svc.Submit(c.ID, p.ID, u2.ID, "cpp", "code u2 late")
	if err == nil || !strings.Contains(err.Error(), "exam time has ended") {
		t.Fatalf("expected expired error for u2 after extension, got %v", err)
	}
}

func TestNoticeAndBroadcastMessages(t *testing.T) {
	svc, _ := setupTestService(t)
	c, _ := svc.CreateContest("NOIP 2026", "", "oi")
	u1, _ := svc.CreateUser("Evan", "student")
	u2, _ := svc.CreateUser("Frank", "student")

	// 更新置顶通告
	cUpdated, err := svc.UpdateNotice(c.ID, "注意：T3 样例第三行输出已更正！")
	if err != nil {
		t.Fatalf("UpdateNotice failed: %v", err)
	}
	if cUpdated.Notice != "注意：T3 样例第三行输出已更正！" {
		t.Fatalf("expected notice updated, got: %s", cUpdated.Notice)
	}

	// 教师发送全员广播
	msgAll, err := svc.SendBroadcastMessage(c.ID, "主考官", "all", "全场通知", "离考试结束还有 30 分钟，请及时保存代码并检查文件名", "warning")
	if err != nil {
		t.Fatalf("SendBroadcastMessage failed: %v", err)
	}
	if msgAll.ID == "" || msgAll.Level != "warning" {
		t.Fatalf("invalid broadcast message: %+v", msgAll)
	}

	// 教师发送定向通知给 u1
	_, err = svc.SendBroadcastMessage(c.ID, "监考员A", u1.ID, "座位提醒", "你的机位屏幕存在反光，已拉窗帘", "info")
	if err != nil {
		t.Fatalf("Send targeted message failed: %v", err)
	}

	// 查询 u1 视角的消息列表 -> 应该包含全员消息 + 针对 u1 的消息（共2条）
	msgs1, err := svc.ListBroadcastMessages(c.ID, u1.ID)
	if err != nil {
		t.Fatalf("ListBroadcastMessages for u1 failed: %v", err)
	}
	if len(msgs1) != 2 {
		t.Fatalf("expected 2 messages for u1, got %d", len(msgs1))
	}

	// 查询 u2 视角的消息列表 -> 应该只包含全员消息（共1条）
	msgs2, err := svc.ListBroadcastMessages(c.ID, u2.ID)
	if err != nil {
		t.Fatalf("ListBroadcastMessages for u2 failed: %v", err)
	}
	if len(msgs2) != 1 {
		t.Fatalf("expected 1 message for u2, got %d", len(msgs2))
	}
}

func TestGESPObjectiveQuestionsInstantGrading(t *testing.T) {
	svc, _ := setupTestService(t)
	c, _ := svc.CreateContest("GESP Grade 2 Objective", "", "oi")
	u, _ := svc.CreateUser("Grace", "student")
	_ = svc.AddParticipantToContest(c.ID, u.ID)

	// 创建客观题试卷（含单选题、判断题、多选题）
	choiceProb, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID,
		Code:      "GESP01",
		Title:     "GESP 二级客观选择题全卷",
		Statement: "请完成下列客观题并在答题卡上作答：",
		TaskType:  domain.TaskChoice,
		ChoiceQuestions: []domain.ChoiceQuestion{
			{
				ID:      1,
				Type:    "single",
				Stem:    "下列哪个是 C++ 的关键字？\nA. while\nB. integer\nC. main\nD. string",
				Options: []string{"A. while", "B. integer", "C. main", "D. string"},
				Answer:  "A",
				Score:   2,
			},
			{
				ID:     2,
				Type:   "judge",
				Stem:   "表达式 5 / 2 在 C++ 中的值为 2.5。",
				Answer: "F",
				Score:  2,
			},
			{
				ID:      3,
				Type:    "multiple",
				Stem:    "下列属于基础整型数据类型的有（多选）：\nA. int\nB. float\nC. short\nD. double",
				Options: []string{"A. int", "B. float", "C. short", "D. double"},
				Answer:  []string{"A", "C"},
				Score:   4,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateProblem choice failed: %v", err)
	}
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatalf("start contest: %v", err)
	}

	// 考生提交全对答题卡（JSON）
	fullAnswers := map[string]any{
		"1": "A",
		"2": "错",                // 支持 "错" / "F" 归一化
		"3": []string{"C", "A"}, // 支持无序多选归一化
	}
	fullJSON, _ := json.Marshal(fullAnswers)

	subFull, err := svc.Submit(c.ID, choiceProb.ID, u.ID, "json", string(fullJSON))
	if err != nil {
		t.Fatalf("Submit full answers failed: %v", err)
	}

	// 纯内存微秒级直接完成评测！无需轮询！
	if subFull.Verdict != domain.VerdictAccepted {
		t.Fatalf("expected accepted for full answers, got %s", subFull.Verdict)
	}
	if subFull.Score != 8 {
		t.Fatalf("expected score 8, got %d", subFull.Score)
	}
	if len(subFull.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(subFull.Cases))
	}
	for i, cRes := range subFull.Cases {
		if cRes.Verdict != domain.CaseAC {
			t.Fatalf("case %d expected AC, got %s", i+1, cRes.Verdict)
		}
	}

	// 考生提交错了一题的答题卡
	partialAnswers := map[string]any{
		"1": "B",     // 错
		"2": "false", // 对
		"3": "AC",    // 对 (字符串格式)
	}
	partialJSON, _ := json.Marshal(partialAnswers)
	subPart, err := svc.Submit(c.ID, choiceProb.ID, u.ID, "json", string(partialJSON))
	if err != nil {
		t.Fatalf("Submit partial answers failed: %v", err)
	}
	if subPart.Verdict != domain.VerdictWrongAnswer {
		t.Fatalf("expected WA for partial answers, got %s", subPart.Verdict)
	}
	if subPart.Score != 6 { // 2 + 4 = 6
		t.Fatalf("expected score 6, got %d", subPart.Score)
	}
	if subPart.Cases[0].Verdict != domain.CaseWA || subPart.Cases[0].Score != 0 {
		t.Fatalf("case 1 should be WA with score 0, got %+v", subPart.Cases[0])
	}
}

func TestChoiceKeyJSONIsSynchronizedOnCreateAndUpdate(t *testing.T) {
	svc, _ := setupTestService(t)
	c, _ := svc.CreateContest("Choice keys", "", "oi")

	created, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID,
		Code:      "Q1",
		Title:     "Objective",
		Statement: "Answer all questions.",
		TaskType:  domain.TaskChoice,
		ChoiceQuestions: []domain.ChoiceQuestion{
			{ID: 1, Type: "single", Stem: "One", Options: []string{"A. x", "B. y"}, Answer: "A", Score: 2},
		},
	})
	if err != nil {
		t.Fatalf("create choice problem: %v", err)
	}
	if created.ChoiceKeyJSON != `{"1":"A"}` {
		t.Fatalf("choice key not synchronized on create: %s", created.ChoiceKeyJSON)
	}

	updated, err := svc.UpdateProblem(domain.Problem{
		ID:        created.ID,
		ContestID: c.ID,
		ChoiceQuestions: []domain.ChoiceQuestion{
			{ID: 1, Type: "single", Stem: "One", Options: []string{"A. x", "B. y"}, Answer: "B", Score: 2},
			{ID: 2, Type: "judge", Stem: "Two", Answer: "T", Score: 2},
		},
	})
	if err != nil {
		t.Fatalf("update choice problem: %v", err)
	}
	if updated.ChoiceKeyJSON != `{"1":"B","2":"T"}` && updated.ChoiceKeyJSON != `{"2":"T","1":"B"}` {
		t.Fatalf("choice key not synchronized on update: %s", updated.ChoiceKeyJSON)
	}

	if _, err := svc.UpdateProblem(domain.Problem{
		ID:        created.ID,
		ContestID: c.ID,
		ChoiceQuestions: []domain.ChoiceQuestion{
			{ID: 1, Type: "single", Stem: "One", Options: []string{"A. x", "B. y"}, Answer: "Z", Score: 2},
		},
	}); err == nil {
		t.Fatal("invalid updated answer should be rejected")
	}
}

func TestDeleteProblemDetachesContestAndKeepsSubmissions(t *testing.T) {
	svc, _ := setupTestService(t)
	u, _ := svc.CreateUser("Alice", "student")
	c, _ := svc.CreateContest("Deletion", "", "oi")
	if err := svc.AddParticipantToContest(c.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	// A historical submission may survive a draft-time paper cleanup. Insert
	// it directly because the public submit path correctly refuses draft contests.
	historical, err := svc.Store().CreateSubmission(domain.Submission{
		ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Language: "cpp",
		Code: "int main(){}", Verdict: domain.VerdictQueued, SubmittedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteProblem(c.ID, p.ID); err != nil {
		t.Fatalf("delete problem: %v", err)
	}
	if ps := svc.ListProblems(c.ID); len(ps) != 0 {
		t.Fatalf("problem still listed: %+v", ps)
	}
	contest, err := svc.GetContest(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contest.ProblemIDs) != 0 {
		t.Fatalf("contest manifest still references deleted problem: %+v", contest.ProblemIDs)
	}
	if _, err := svc.GetSubmission(historical.ID); err != nil {
		t.Fatalf("historical submission should be retained: %v", err)
	}
	if err := svc.DeleteProblem(c.ID, p.ID); err == nil {
		t.Fatal("deleting a missing problem must fail")
	}
}

func TestZipPasswordVerification(t *testing.T) {
	svc, _ := setupTestService(t)
	c, _ := svc.CreateContest("Protected Contest", "", "oi")

	// 未设密码时直接通过
	ok, err := svc.VerifyZipPassword(c.ID, "any")
	if err != nil || !ok {
		t.Fatalf("expected empty password to allow any input, got %v, %v", ok, err)
	}

	// 设定密码
	_, err = svc.SetZipPassword(c.ID, "NOIP-2026-PASSWORD")
	if err != nil {
		t.Fatalf("SetZipPassword failed: %v", err)
	}

	// 输错密码
	ok, err = svc.VerifyZipPassword(c.ID, "wrong-password")
	if err != nil || ok {
		t.Fatalf("expected wrong password to fail, got ok=%v", ok)
	}

	// 输对密码
	ok, err = svc.VerifyZipPassword(c.ID, "NOIP-2026-PASSWORD")
	if err != nil || !ok {
		t.Fatalf("expected correct password to succeed, got ok=%v", ok)
	}
}
