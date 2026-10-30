package packageio

import (
	"archive/zip"
	"errors"

	"github.com/yviscool/forge/internal/domain"
)

// Format 表示题包格式类型。
type Format string

const (
	FormatPolygon Format = "polygon" // Codeforces Polygon 题包 (problem.xml)
	FormatHydro   Format = "hydro"   // Hydro OJ 题包 (problem.yaml)
	FormatQDUOJ   Format = "qduoj"   // QDUOJ / 洛谷标准 JSON 题包 (problem.json)
	FormatFPS     Format = "fps"     // HUSTOJ / Free Problem Set (fps.xml)
	FormatUnknown Format = "unknown"
)

// ParsedPackage 表示解析后的通用题包结果。
type ParsedPackage struct {
	Format      Format
	Problem     domain.Problem
	TestCases   []domain.TestCase
	InFiles     map[string][]byte // 文件名 -> 字节内容
	OutFiles    map[string][]byte // 文件名 -> 字节内容
	CheckerCode string
	CheckerLang string
}

// ErrUnsupportedFormat 未知或不支持的题包格式。
var ErrUnsupportedFormat = errors.New("unsupported problem package format")

// MaxZipEntries 单压缩包最多文件数（防止 Zip 炸弹）。
const MaxZipEntries = 2048

// MaxSingleFileSize 单文件最大解压大小 (32MB)。
const MaxSingleFileSize = 32 << 20

// MaxTotalPackageSize 整个题包最大解压大小 (256MB)。
const MaxTotalPackageSize = 256 << 20

// DetectFormat 嗅探 ZIP 归档中的题包格式。
func DetectFormat(r *zip.Reader) Format {
	for _, f := range r.File {
		name := cleanZipPath(f.Name)
		switch {
		case name == "problem.xml":
			return FormatPolygon
		case name == "problem.yaml" || name == "problem.yml":
			return FormatHydro
		case name == "problem.json":
			return FormatQDUOJ
		case name == "fps.xml" || (len(name) > 4 && name[len(name)-4:] == ".xml"):
			return FormatFPS
		}
	}
	return FormatUnknown
}

func cleanZipPath(p string) string {
	for len(p) > 0 && (p[0] == '/' || p[0] == '\\') {
		p = p[1:]
	}
	return p
}
