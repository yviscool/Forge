package judge

import (
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestCSP_Distance(t *testing.T) {
	all := loadCases(t, "distance", "distance", allIdx(10))

	t.Run("ac", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_ac.cpp"), all, stdLimits())
		assertVerdicts(t, "distance/ac", got, wantAll(domain.CaseAC, 10))
	})

	t.Run("wa_nosort", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_wa_nosort.cpp"), all, stdLimits())
		// 未排序输入挂（有序点恰好能过；9 恰为有序 coincidental pass）。
		assertVerdicts(t, "distance/wa_nosort", got, []domain.CaseVerdict{
			domain.CaseAC, domain.CaseWA, domain.CaseAC, domain.CaseAC, domain.CaseWA,
			domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC,
		})
	})

	t.Run("wa_strict", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_wa_strict.cpp"), all, stdLimits())
		// check 用 > 代替 >=：答案系统性偏小（恰好整除边界挂）。
		assertVerdicts(t, "distance/wa_strict", got, []domain.CaseVerdict{
			domain.CaseWA, domain.CaseWA, domain.CaseAC, domain.CaseWA, domain.CaseWA,
			domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA,
		})
	})

	t.Run("wa_formula", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_wa_formula.cpp"), all, stdLimits())
		// 乱猜公式：仅 case7 巧合通过。
		assertVerdicts(t, "distance/wa_formula", got, []domain.CaseVerdict{
			domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA, domain.CaseWA,
			domain.CaseWA, domain.CaseAC, domain.CaseWA, domain.CaseWA, domain.CaseWA,
		})
	})

	t.Run("re", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_re.cpp"), all, stdLimits())
		assertVerdicts(t, "distance/re", got, wantAll(domain.CaseRE, 10))
	})

	t.Run("ce", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "distance_ce.cpp"), all, stdLimits())
		assertVerdicts(t, "distance/ce", got, wantAll(domain.CaseCE, 10))
	})

	t.Run("slow_case10", func(t *testing.T) {
		big := loadCases(t, "distance", "distance", []int{10})
		got := runVariant(t, "cpp", loadCode(t, "distance_slow.cpp"), big, Limits{TimeMs: 3000, MemoryMiB: 256})
		assertVerdicts(t, "distance/slow", got, []domain.CaseVerdict{domain.CaseTLE})
	})
}
