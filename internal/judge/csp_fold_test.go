package judge

import (
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestCSP_Fold(t *testing.T) {
	all := loadCases(t, "fold", "fold", allIdx(10))

	t.Run("ac", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "fold_ac.cpp"), all, stdLimits())
		assertVerdicts(t, "fold/ac", got, wantAll(domain.CaseAC, 10))
	})

	t.Run("wa_single", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "fold_wa_single.cpp"), all, stdLimits())
		// 长度1的段多算一位：单字符/全交替点挂。
		assertVerdicts(t, "fold/wa_single", got, []domain.CaseVerdict{
			domain.CaseWA, domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC,
			domain.CaseAC, domain.CaseWA, domain.CaseWA, domain.CaseAC, domain.CaseAC,
		})
	})

	t.Run("wa_digit", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "fold_wa_digit.cpp"), all, stdLimits())
		// 两位数以上的重复次数少算：多位数点挂。
		assertVerdicts(t, "fold/wa_digit", got, []domain.CaseVerdict{
			domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseAC, domain.CaseWA,
			domain.CaseWA, domain.CaseAC, domain.CaseAC, domain.CaseWA, domain.CaseWA,
		})
	})

	t.Run("re", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "fold_re.cpp"), all, stdLimits())
		assertVerdicts(t, "fold/re", got, wantAll(domain.CaseRE, 10))
	})

	t.Run("ce", func(t *testing.T) {
		got := runVariant(t, "cpp", loadCode(t, "fold_ce.cpp"), all, stdLimits())
		assertVerdicts(t, "fold/ce", got, wantAll(domain.CaseCE, 10))
	})

	t.Run("slow_case10", func(t *testing.T) {
		big := loadCases(t, "fold", "fold", []int{10})
		got := runVariant(t, "cpp", loadCode(t, "fold_slow.cpp"), big, Limits{TimeMs: 1000, MemoryMiB: 256})
		assertVerdicts(t, "fold/slow", got, []domain.CaseVerdict{domain.CaseTLE})
	})
}
