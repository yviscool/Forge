package domain

import "testing"

func TestProvincesBuiltinTable(t *testing.T) {
	ps := Provinces()
	if len(ps) != 34 {
		t.Fatalf("builtin provinces = %d, want 34", len(ps))
	}
	// 排序必须严格递增且与表内声明一致（CSP 考务材料按此顺序排印）。
	prev := 0
	for _, p := range ps {
		if p.Sort <= prev {
			t.Fatalf("province %s sort %d not ascending after %d", p.Code, p.Sort, prev)
		}
		prev = p.Sort
	}
	if ps[0].Code != "BJ" || ps[0].Name != "北京" {
		t.Fatalf("first province = %+v, want BJ 北京", ps[0])
	}
	if p, ok := ProvinceByCode("ha"); !ok || p.Code != "HA" || p.Name != "河南" {
		t.Fatalf("ProvinceByCode(ha) = %+v ok=%v", p, ok)
	}
	if _, ok := ProvinceByName("内蒙古"); !ok {
		t.Fatal("ProvinceByName(内蒙古) should match NM")
	}
	if _, ok := ProvinceByName("河北"); !ok {
		t.Fatal("ProvinceByName(河北) should match HE, not collide with 湖北/湖南")
	}
	// 返回的是副本，调用方修改不得污染内置表。
	ps[0].Code = "XX"
	if p, _ := ProvinceByCode("BJ"); p.Name != "北京" {
		t.Fatal("Provinces() must return a copy")
	}
}

func TestNormalizeProvinceRef(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"HA", "HA", false},
		{"ha", "HA", false},
		{"  河南  ", "HA", false},
		{"黑龙江", "HL", false},
		{"usa", "", true},
		{"河北北", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeProvinceRef(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeProvinceRef(%q) = %q, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeProvinceRef(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeProvinceRef(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCSPTicket(t *testing.T) {
	if got := FormatCSPTicket("ha", "j", 1, 5); got != "HA-J00001" {
		t.Fatalf("FormatCSPTicket = %q, want HA-J00001", got)
	}
	if got := FormatCSPTicket("GD", "S", 42, 6); got != "GD-S000042" {
		t.Fatalf("FormatCSPTicket = %q, want GD-S000042", got)
	}
	// 序号超出位宽自然升位，绝不回绕重号。
	if got := FormatCSPTicket("HA", "J", 123456, 5); got != "HA-J123456" {
		t.Fatalf("overflow ticket = %q, want HA-J123456", got)
	}
	// 位宽低于下限时按 3 位处理。
	if got := FormatCSPTicket("HA", "J", 7, 1); got != "HA-J007" {
		t.Fatalf("clamped ticket = %q, want HA-J007", got)
	}
}

func TestParseCSPTicketNumber(t *testing.T) {
	if n, ok := ParseCSPTicketNumber("HA-J00042", "HA", "J"); !ok || n != 42 {
		t.Fatalf("parse HA-J00042 = %d ok=%v, want 42", n, ok)
	}
	// 小写存储的票号同样可解析（归一化为大写）。
	if n, ok := ParseCSPTicketNumber("ha-j7", "HA", "J"); !ok || n != 7 {
		t.Fatalf("parse ha-j7 = %d ok=%v, want 7", n, ok)
	}
	bad := []struct{ ticket, prov, group string }{
		{"HA-S00001", "HA", "J"}, // 组别不符
		{"GD-J00001", "HA", "J"}, // 省份不符
		{"HA-J", "HA", "J"},      // 缺序号
		{"HA-J0000x", "HA", "J"}, // 非数字
		{"HA-J-0001", "HA", "J"}, // 序号夹符号
		{"", "HA", "J"},          // 空票号
		{"HAJ00001", "HA", "J"},  // 缺分隔符
	}
	for _, c := range bad {
		if _, ok := ParseCSPTicketNumber(c.ticket, c.prov, c.group); ok {
			t.Errorf("ParseCSPTicketNumber(%q,%s,%s) should not parse", c.ticket, c.prov, c.group)
		}
	}
}

func TestTicketGroupHelpers(t *testing.T) {
	if !ValidTicketGroup("j") || !ValidTicketGroup("S") {
		t.Fatal("J/S must be valid groups")
	}
	if ValidTicketGroup("A") || ValidTicketGroup("") {
		t.Fatal("other groups must be rejected")
	}
	if NormalizeTicketGroup(" s ") != "S" {
		t.Fatal("NormalizeTicketGroup should trim and uppercase")
	}
}
