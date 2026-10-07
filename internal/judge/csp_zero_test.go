package judge

import (
	"os/exec"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestCSP_Zero(t *testing.T) {
	all := loadCases(t, "zero", "zero", allIdx(10))

	t.Run("ac", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "zero_ac.cpp"), all, stdLimits())
		assertVerdicts(t, "zero/ac", got, wantAll(domain.CaseAC, 10))
	})

	t.Run("wa_mindigit", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "zero_wa_mindigit.cpp"), all, stdLimits())
		// 减最小数位：case2 巧合通过，其余步数偏多。
		assertVerdicts(t, "zero/wa_mindigit", got, []domain.CaseVerdict{
			domain.CaseAC, domain.CaseAC, domain.CaseWA, domain.CaseAC, domain.CaseWA,
			domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA,
		})
	})

	t.Run("wa_dpinit", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "zero_wa_dpinit.cpp"), all, stdLimits())
		// dp[0]=1：答案整体+1，全挂。
		assertVerdicts(t, "zero/wa_dpinit", got, wantAll(domain.CaseWA, 10))
	})

	t.Run("re", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "zero_re.cpp"), all, stdLimits())
		assertVerdicts(t, "zero/re", got, wantAll(domain.CaseRE, 10))
	})

	t.Run("ce", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "zero_ce.cpp"), all, stdLimits())
		assertVerdicts(t, "zero/ce", got, wantAll(domain.CaseCE, 10))
	})

	t.Run("slow_case10", func(t *testing.T) {
		big := loadCases(t, "zero", "zero", []int{10})
		got := runVariant(t, "cpp", loadCode(t, "zero_slow.cpp"), big, Limits{TimeMs: 1000, MemoryMiB: 512})
		assertVerdicts(t, "zero/slow", got, []domain.CaseVerdict{domain.CaseTLE})
	})

	t.Run("ac_python_small", func(t *testing.T) {
		if _, err := exec.LookPath("python"); err != nil {
			t.Skip("python not found")
		}
		small := loadCases(t, "zero", "zero", []int{1, 2, 3, 4, 5, 6})
		got := runVariant(t, "python", loadCode(t, "zero_ac.py"), small, Limits{TimeMs: 10000, MemoryMiB: 512})
		assertVerdicts(t, "zero/ac_py", got, wantAll(domain.CaseAC, 6))
	})

	t.Run("slow_python_custom", func(t *testing.T) {
		if _, err := exec.LookPath("python"); err != nil {
			t.Skip("python not found")
		}
		// 无记忆递归在 n=27 即指数爆炸（与 corpus 无关的定制点）。
		small := loadCases(t, "zero", "zero", []int{3})
		got := runVariant(t, "python", loadCode(t, "zero_slow.py"), small, Limits{TimeMs: 3000, MemoryMiB: 512})
		assertVerdicts(t, "zero/slow_py", got, []domain.CaseVerdict{domain.CaseTLE})
	})
}
