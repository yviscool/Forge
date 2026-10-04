package i18n

import "testing"

func TestNegotiateAndVerdict(t *testing.T) {
	if Negotiate("en-US,en;q=0.9") != "en-US" {
		t.Fatal("negotiate broken")
	}
	if Negotiate("") != "zh-CN" {
		t.Fatal("default should be zh-CN")
	}
	if VerdictText("accepted", "zh-CN") != "答案正确" {
		t.Fatal("verdict zh broken")
	}
	if ErrorText("contest_not_running", "en-US") == "" {
		t.Fatal("error text broken")
	}
}
