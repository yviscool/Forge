package judge

import (
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestCSP_Point(t *testing.T) {
	all := loadCases(t, "point", "point", allIdx(10))

	t.Run("ac", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "point_ac.cpp"), all, stdLimits())
		assertVerdicts(t, "point/ac", got, wantAll(domain.CaseAC, 10))
	})

	t.Run("wa_merge", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "point_wa_merge.cpp"), all, stdLimits())
		// 连通块计数：相接/嵌套/链式区间少算。
		assertVerdicts(t, "point/wa_merge", got, []domain.CaseVerdict{
			domain.CaseAC, domain.CaseAC, domain.CaseWA, domain.CaseAC, domain.CaseWA,
			domain.CaseAC, domain.CaseWA, domain.CaseAC, domain.CaseWA, domain.CaseAC,
		})
	})

	t.Run("wa_left", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "point_wa_left.cpp"), all, stdLimits())
		// 按左端点贪心：嵌套区间翻车。
		assertVerdicts(t, "point/wa_left", got, []domain.CaseVerdict{
			domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseWA,
			domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC,
		})
	})

	t.Run("re", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "point_re.cpp"), all, stdLimits())
		assertVerdicts(t, "point/re", got, wantAll(domain.CaseRE, 10))
	})

	t.Run("ce", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "point_ce.cpp"), all, stdLimits())
		assertVerdicts(t, "point/ce", got, wantAll(domain.CaseCE, 10))
	})

	t.Run("slow_case10", func(t *testing.T) {
		big := loadCases(t, "point", "point", []int{10})
		got := runVariant(t, "cpp", loadCode(t, "point_slow.cpp"), big, Limits{TimeMs: 1000, MemoryMiB: 256})
		assertVerdicts(t, "point/slow", got, []domain.CaseVerdict{domain.CaseTLE})
	})
}
