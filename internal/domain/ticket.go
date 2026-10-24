package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Ticket 是考生在某个赛季、某个组别下的准考证号。
//
// 设计要点：准考证是「身份」，不是「场次」。同一个赛季内，考生参加多少场
// 模拟赛/正式赛都用同一个号——这与 CCF 报名发号的真实流程一致，也让学生
// 的答题文件夹名在整个赛季里保持不变。
//
//	唯一键：(userID, season, group)
//
// 号本身仍沿用 CSP 格式：省份-组别+定长序号（HA-J00001），由
// FormatCSPTicket 生成，序号在（赛季, 组别）内递增。
type Ticket struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	Season   string `json:"season"`
	Group    string `json:"group"`
	Province string `json:"province"`
	Number   string `json:"number"`
	// CreatedAt 发号时间，用于按发号顺序稳定排序。
	CreatedAt time.Time `json:"createdAt"`
}

// MaxSeasonRunes 限制赛季标识长度（如 "2026"、"2026秋"）。
const MaxSeasonRunes = 16

// NormalizeSeason 归一化赛季标识：去空白，空值回落为给定年份。
func NormalizeSeason(season string) string {
	return strings.TrimSpace(season)
}

// ValidateSeason 校验赛季标识：非空、长度受限、不含路径分隔符。
func ValidateSeason(season string) error {
	s := NormalizeSeason(season)
	if s == "" {
		return errors.New("season is required")
	}
	if len([]rune(s)) > MaxSeasonRunes {
		return fmt.Errorf("season exceeds %d characters", MaxSeasonRunes)
	}
	if strings.ContainsAny(s, `/\:*?"<>|`) {
		return errors.New("season cannot contain path separators")
	}
	return nil
}

// CurrentSeason 返回默认赛季标识（当前年份）。
func CurrentSeason(now time.Time) string {
	return fmt.Sprintf("%d", now.Year())
}

// NormalizeTicket 归一化准考证：赛季去空白、组别与号码大写。存储层在
// 落库前调用，保证唯一索引（sqlite 默认大小写敏感）与内存实现按同一
// 口径判重。
func NormalizeTicket(t Ticket) Ticket {
	t.Season = NormalizeSeason(t.Season)
	t.Group = NormalizeTicketGroup(t.Group)
	t.Number = strings.ToUpper(strings.TrimSpace(t.Number))
	t.Province = strings.ToUpper(strings.TrimSpace(t.Province))
	return t
}

// TicketKey 报告一张准考证是否属于给定的（赛季, 组别）。
func (t Ticket) BelongsTo(season, group string) bool {
	return strings.EqualFold(NormalizeSeason(t.Season), NormalizeSeason(season)) &&
		NormalizeTicketGroup(t.Group) == NormalizeTicketGroup(group)
}

// ParseTicket 从准考证号反解出省份、组别与序号（HA-J00001 → HA, J, 1）。
// 老数据迁移与"号从哪来"的展示都靠它。
func ParseTicket(ticket string) (province, group string, number int, ok bool) {
	s := strings.ToUpper(strings.TrimSpace(ticket))
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 || len(parts[1]) < 2 {
		return "", "", 0, false
	}
	province = parts[0]
	group = parts[1][:1]
	if !ValidProvinceCode(province) || !ValidTicketGroup(group) {
		return "", "", 0, false
	}
	n, okNumber := ParseCSPTicketNumber(s, province, group)
	if !okNumber {
		return "", "", 0, false
	}
	return province, group, n, true
}
