package domain

import (
	"fmt"
	"strings"
)

// Province 内置省级行政区划引用数据：Code 为 CSP 准考证号的省份编号
// （如 HA-J00001 中的 HA），Sort 决定下拉与列表的固定展示顺序。
// 行政区划是事实数据，不随部署变化，因此编译进二进制；
// 各省的启用/停用属于考务运营状态，由 Store 持久化（ports.Store.ProvinceStates）。
type Province struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// provinces 顺序与编号遵循全国省级行政区惯例排列（直辖市 → 各省 → 自治区 → 特区/台湾）。
var provinces = []Province{
	{"BJ", "北京", 10},
	{"SH", "上海", 20},
	{"TJ", "天津", 30},
	{"CQ", "重庆", 40},
	{"HE", "河北", 50},
	{"SX", "山西", 60},
	{"NM", "内蒙古", 70},
	{"LN", "辽宁", 80},
	{"JL", "吉林", 90},
	{"HL", "黑龙江", 100},
	{"JS", "江苏", 110},
	{"ZJ", "浙江", 120},
	{"AH", "安徽", 130},
	{"FJ", "福建", 140},
	{"JX", "江西", 150},
	{"SD", "山东", 160},
	{"HA", "河南", 170},
	{"HB", "湖北", 180},
	{"HN", "湖南", 190},
	{"GD", "广东", 200},
	{"GX", "广西", 210},
	{"HI", "海南", 220},
	{"SC", "四川", 230},
	{"GZ", "贵州", 240},
	{"YN", "云南", 250},
	{"SN", "陕西", 260},
	{"GS", "甘肃", 270},
	{"QH", "青海", 280},
	{"NX", "宁夏", 290},
	{"XJ", "新疆", 300},
	{"XZ", "西藏", 310},
	{"HK", "香港", 320},
	{"MO", "澳门", 330},
	{"TW", "台湾", 340},
}

// Provinces 返回按 Sort 升序的内置省份表副本。
func Provinces() []Province {
	out := make([]Province, len(provinces))
	copy(out, provinces)
	return out
}

// ProvinceByCode 按大写编号查省份。
func ProvinceByCode(code string) (Province, bool) {
	for _, p := range provinces {
		if p.Code == strings.ToUpper(strings.TrimSpace(code)) {
			return p, true
		}
	}
	return Province{}, false
}

// ProvinceByName 按中文名精确匹配省份（CSV 导入允许直接写「河南」）。
func ProvinceByName(name string) (Province, bool) {
	for _, p := range provinces {
		if p.Name == strings.TrimSpace(name) {
			return p, true
		}
	}
	return Province{}, false
}

// ValidProvinceCode 报告 code 是否为内置省份编号（大小写不敏感，内部会归一化为大写）。
func ValidProvinceCode(code string) bool {
	_, ok := ProvinceByCode(code)
	return ok
}

// NormalizeProvinceRef 把用户输入的省份（编号大小写不敏感，或中文全名）
// 归一化为大写编号；未知取值返回错误而不是静默置空——准考证号直接进证书链路。
func NormalizeProvinceRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	if p, ok := ProvinceByCode(trimmed); ok {
		return p.Code, nil
	}
	if p, ok := ProvinceByName(trimmed); ok {
		return p.Code, nil
	}
	return "", fmt.Errorf("unknown province %q", trimmed)
}

// CSP 准考证号组别：J=入门级，S=提高级。
const (
	TicketGroupJunior = "J"
	TicketGroupSenior = "S"
	// DefaultTicketDigits 缺省序号位宽（CSP 官方样式为 5 位：HA-J00001）。
	DefaultTicketDigits = 5
	// MinTicketDigits / MaxTicketDigits 允许的位宽范围。
	MinTicketDigits = 3
	MaxTicketDigits = 8
)

// ValidTicketGroup 报告组别取值是否合法。
func ValidTicketGroup(group string) bool {
	switch strings.ToUpper(strings.TrimSpace(group)) {
	case TicketGroupJunior, TicketGroupSenior:
		return true
	default:
		return false
	}
}

// NormalizeTicketGroup 归一化组别为大写。
func NormalizeTicketGroup(group string) string {
	return strings.ToUpper(strings.TrimSpace(group))
}

// FormatCSPTicket 生成 CSP 准考证号：省份编号-组别+定长序号，如 HA-J00001。
// 序号超出位宽时自然升位（5 位放不下给 6 位），保证不回绕重号。
func FormatCSPTicket(province, group string, number, digits int) string {
	if digits < MinTicketDigits {
		digits = MinTicketDigits
	}
	return fmt.Sprintf("%s-%s%0*d", strings.ToUpper(strings.TrimSpace(province)),
		strings.ToUpper(strings.TrimSpace(group)), digits, number)
}

// CSPTicketPrefix 返回某省某组的准考证前缀（HA-J），用于按序号扫描既有票号。
func CSPTicketPrefix(province, group string) string {
	return strings.ToUpper(strings.TrimSpace(province)) + "-" + strings.ToUpper(strings.TrimSpace(group))
}

// ParseCSPTicketNumber 从既有的本省本组准考证号里解析出序号；
// 格式不匹配（其他省、其他组、缺序号）返回 ok=false。
func ParseCSPTicketNumber(ticket, province, group string) (int, bool) {
	rest := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(ticket)), CSPTicketPrefix(province, group))
	if rest == "" {
		return 0, false
	}
	for _, ch := range rest {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	n := 0
	for _, ch := range rest {
		n = n*10 + int(ch-'0')
		if n > 1<<30 {
			return 0, false
		}
	}
	if n < 1 {
		return 0, false
	}
	return n, true
}
