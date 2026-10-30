package packageio

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

// ExportProblemNOIPackage 生成单题符合 NOI/CSP 标准的离线试题包 ZIP 字节流。
func ExportProblemNOIPackage(p domain.Problem, inFiles, outFiles map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	slug := p.Slug
	if slug == "" {
		slug = strings.ToLower(p.Code)
	}
	if slug == "" {
		slug = "problem"
	}

	// 1. 题面 Markdown
	stmt := fmt.Sprintf("# %s\n\n## 题目描述\n\n%s\n\n## 输入格式\n\n%s\n\n## 输出格式\n\n%s\n\n## 限制说明\n\n%s\n\n%s\n",
		p.Title, p.Statement, p.Input, p.Output, p.Constraints, p.Examples)
	fStmt, err := zw.Create(fmt.Sprintf("%s/problem.md", slug))
	if err != nil {
		return nil, err
	}
	if _, err := fStmt.Write([]byte(stmt)); err != nil {
		return nil, err
	}

	// 2. 评测配置 problem.conf
	conf := fmt.Sprintf("use_builtin_judger=on\ntime_limit=%d\nmemory_limit=%d\nn_tests=%d\ninput_pre=%s\ninput_suf=in\noutput_suf=out\n",
		(p.TimeLimitMs+999)/1000, p.MemoryLimitMiB, len(p.TestCases), slug)
	fConf, err := zw.Create(fmt.Sprintf("%s/problem.conf", slug))
	if err != nil {
		return nil, err
	}
	if _, err := fConf.Write([]byte(conf)); err != nil {
		return nil, err
	}

	// 3. 评测数据 data/
	for idx, tc := range p.TestCases {
		inName := tc.InputFile
		outName := tc.OutputFile
		inData, inOk := inFiles[inName]
		outData, outOk := outFiles[outName]

		dstIn := fmt.Sprintf("%s/data/%s%d.in", slug, slug, idx+1)
		dstOut := fmt.Sprintf("%s/data/%s%d.out", slug, slug, idx+1)

		if inOk {
			f, err := zw.Create(dstIn)
			if err != nil {
				return nil, err
			}
			if _, err := f.Write(inData); err != nil {
				return nil, err
			}
		}
		if outOk {
			f, err := zw.Create(dstOut)
			if err != nil {
				return nil, err
			}
			if _, err := f.Write(outData); err != nil {
				return nil, err
			}
		}
	}

	// 4. 下发文件 down/ (第一组测试点作为选手样例)
	if len(p.TestCases) > 0 {
		firstTc := p.TestCases[0]
		if inData, ok := inFiles[firstTc.InputFile]; ok {
			f, err := zw.Create(fmt.Sprintf("%s/down/%s.in", slug, slug))
			if err == nil {
				_, _ = f.Write(inData)
			}
		}
		if outData, ok := outFiles[firstTc.OutputFile]; ok {
			f, err := zw.Create(fmt.Sprintf("%s/down/%s.out", slug, slug))
			if err == nil {
				_, _ = f.Write(outData)
			}
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportContestNOIPackage 将整场比赛的所有题目打包为符合 NOI/CSP 标准的离线归档。
func ExportContestNOIPackage(c domain.Contest, problems []domain.Problem, allInFiles, allOutFiles map[string]map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, p := range problems {
		slug := p.Slug
		if slug == "" {
			slug = strings.ToLower(p.Code)
		}
		if slug == "" {
			slug = "problem"
		}

		inFiles := allInFiles[p.ID]
		outFiles := allOutFiles[p.ID]

		stmt := fmt.Sprintf("# %s\n\n## 题目描述\n\n%s\n\n## 输入格式\n\n%s\n\n## 输出格式\n\n%s\n\n## 限制说明\n\n%s\n\n%s\n",
			p.Title, p.Statement, p.Input, p.Output, p.Constraints, p.Examples)
		fStmt, _ := zw.Create(fmt.Sprintf("%s/problem.md", slug))
		if fStmt != nil {
			_, _ = fStmt.Write([]byte(stmt))
		}

		conf := fmt.Sprintf("use_builtin_judger=on\ntime_limit=%d\nmemory_limit=%d\nn_tests=%d\ninput_pre=%s\ninput_suf=in\noutput_suf=out\n",
			(p.TimeLimitMs+999)/1000, p.MemoryLimitMiB, len(p.TestCases), slug)
		fConf, _ := zw.Create(fmt.Sprintf("%s/problem.conf", slug))
		if fConf != nil {
			_, _ = fConf.Write([]byte(conf))
		}

		for idx, tc := range p.TestCases {
			dstIn := fmt.Sprintf("%s/data/%s%d.in", slug, slug, idx+1)
			dstOut := fmt.Sprintf("%s/data/%s%d.out", slug, slug, idx+1)
			if inData, ok := inFiles[tc.InputFile]; ok {
				f, _ := zw.Create(dstIn)
				if f != nil {
					_, _ = f.Write(inData)
				}
			}
			if outData, ok := outFiles[tc.OutputFile]; ok {
				f, _ := zw.Create(dstOut)
				if f != nil {
					_, _ = f.Write(outData)
				}
			}
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
