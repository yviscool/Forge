package app

import (
	"archive/zip"
	"errors"
	"fmt"

	"github.com/yviscool/forge/internal/app/packageio"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/testdata"
)

// ImportProblemPackage 解析并导入主流 OJ 格式压缩包（Polygon/Hydro/QDUOJ/FPS），
// 创建题目实体并将测试数据写入文件仓。
func (s *Service) ImportProblemPackage(cid string, r *zip.Reader, fileStore *testdata.Store) (domain.Problem, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.Problem{}, err
	}
	if !c.CanModifySettings() {
		return domain.Problem{}, errors.New("cannot import problem to running or finished contest")
	}

	files, err := packageio.ReadZipEntries(r)
	if err != nil {
		return domain.Problem{}, err
	}

	format := packageio.DetectFormat(r)
	var parsed *packageio.ParsedPackage
	switch format {
	case packageio.FormatPolygon:
		parsed, err = packageio.ParsePolygon(files)
	case packageio.FormatHydro:
		parsed, err = packageio.ParseHydro(files)
	case packageio.FormatQDUOJ:
		parsed, err = packageio.ParseQDUOJ(files)
	case packageio.FormatFPS:
		parsed, err = packageio.ParseFPS(files)
	default:
		return domain.Problem{}, packageio.ErrUnsupportedFormat
	}
	if err != nil {
		return domain.Problem{}, fmt.Errorf("parse %s package: %w", format, err)
	}

	// 自动计算下一题编号 A, B, C, ...
	code := string(rune('A' + len(c.ProblemIDs)))
	if len(c.ProblemIDs) >= 26 {
		code = fmt.Sprintf("P%d", len(c.ProblemIDs)+1)
	}

	prob := parsed.Problem
	prob.ContestID = cid
	prob.Code = code
	if prob.Slug == "" {
		prob.Slug = "p" + code
	}
	prob.UpdatedAt = s.clock.Now()

	// 题目入库
	created, err := s.CreateProblem(prob)
	if err != nil {
		return domain.Problem{}, err
	}

	// 写入测试数据到文件仓
	if fileStore != nil {
		for name, content := range parsed.InFiles {
			_ = fileStore.Put(cid, created.ID, name, content)
		}
		for name, content := range parsed.OutFiles {
			_ = fileStore.Put(cid, created.ID, name, content)
		}
	}

	return created, nil
}

// ExportNOIContestPackage 将指定比赛的所有试题与测试数据导出为符合 NOI/CSP 标准的归档包。
func (s *Service) ExportNOIContestPackage(cid string, fileStore *testdata.Store) ([]byte, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return nil, err
	}
	problems, err := s.store.ListProblems(cid)
	if err != nil {
		return nil, err
	}

	allInFiles := make(map[string]map[string][]byte)
	allOutFiles := make(map[string]map[string][]byte)

	if fileStore != nil {
		for _, p := range problems {
			inMap := make(map[string][]byte)
			outMap := make(map[string][]byte)
			for _, tc := range p.TestCases {
				if tc.InputFile != "" {
					if d, err := fileStore.Get(cid, p.ID, tc.InputFile); err == nil {
						inMap[tc.InputFile] = d
					}
				}
				if tc.OutputFile != "" {
					if d, err := fileStore.Get(cid, p.ID, tc.OutputFile); err == nil {
						outMap[tc.OutputFile] = d
					}
				}
			}
			allInFiles[p.ID] = inMap
			allOutFiles[p.ID] = outMap
		}
	}

	return packageio.ExportContestNOIPackage(c, problems, allInFiles, allOutFiles)
}
