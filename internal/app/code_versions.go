package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// SaveCodeVersion 记录一个代码版本（保存或提交即版本）。
// 语义：
//   - 比赛结束后不再接受新版本（历史只读）；
//   - 与最近一个版本内容+语言完全相同时不追加（自动保存防抖后的幂等）；
//   - 体积与提交上限一致；
//   - 写入后广播 code.saved 事件（只含元数据，不含代码内容——SSE 是共享考场流）。
func (s *Service) SaveCodeVersion(contestID, problemID, userID, language, code, source string) (domain.CodeVersion, error) {
	if strings.TrimSpace(source) == "" {
		source = domain.VersionSourceSave
	}
	if source != domain.VersionSourceSave && source != domain.VersionSourceSubmit {
		return domain.CodeVersion{}, fmt.Errorf("invalid version source %q", source)
	}
	if len(code) > domain.MaxCodeVersionBytes {
		return domain.CodeVersion{}, fmt.Errorf("code exceeds %d bytes", domain.MaxCodeVersionBytes)
	}
	var created domain.CodeVersion
	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.GetContest(contestID)
		if err != nil {
			return err
		}
		if c.Status == string(domain.Finished) {
			return errors.New("contest is finished; version history is read-only")
		}
		if _, err := tx.GetProblem(problemID); err != nil {
			return fmt.Errorf("problem %s: %w", problemID, err)
		}
		// 内容去重：与最近版本完全一致（语言与代码）时复用，不追加 seq。
		recent, err := tx.ListCodeVersions(contestID, problemID, userID, 1)
		if err != nil {
			return err
		}
		if len(recent) == 1 && recent[0].Code == code && recent[0].Language == language {
			created = recent[0]
			return nil
		}
		created, err = tx.CreateCodeVersion(domain.CodeVersion{
			ContestID: contestID, ProblemID: problemID, UserID: userID,
			Language:  language,
			Code:      code,
			Source:    source,
			CreatedAt: s.clock.Now(),
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.CodeVersion{}, err
	}
	if created.ID != "" && source == domain.VersionSourceSave {
		s.emit("code.saved", contestID, map[string]any{
			"versionId": created.ID,
			"problemId": created.ProblemID,
			"userId":    created.UserID,
			"seq":       created.Seq,
			"language":  created.Language,
			"lines":     strings.Count(created.Code, "\n") + 1,
			"at":        created.CreatedAt,
		})
	}
	return created, nil
}

// ListCodeVersions 返回某考生某题的版本历史（最新在前，含代码内容）。
// 学生只能看自己的；教师/管理员可看任意考生（监考与演进对比）。
func (s *Service) ListCodeVersions(actorRole, actorUserID, contestID, problemID, userID string, limit int) ([]domain.CodeVersion, error) {
	if userID == "" {
		return nil, errors.New("userId is required")
	}
	if actorRole != domain.RoleTeacher && actorRole != domain.RoleAdmin {
		if actorUserID == "" || actorUserID != userID {
			return nil, fmt.Errorf("forbidden: students may only list their own versions")
		}
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListCodeVersions(contestID, problemID, userID, limit)
}

// recordSubmitVersion 在提交事务内落一个 submit 来源的版本（失败不阻断提交）。
func (s *Service) recordSubmitVersion(tx ports.Store, x domain.Submission) {
	recent, err := tx.ListCodeVersions(x.ContestID, x.ProblemID, x.UserID, 1)
	if err == nil && len(recent) == 1 && recent[0].Code == x.Code && recent[0].Language == x.Language {
		// 内容与最新保存版本一致：把该版本升级为 submit 来源语义即可，
		// 不额外占用一个序号。
		return
	}
	if _, err := tx.CreateCodeVersion(domain.CodeVersion{
		ContestID: x.ContestID, ProblemID: x.ProblemID, UserID: x.UserID,
		Language: x.Language, Code: x.Code,
		Source: domain.VersionSourceSubmit, CreatedAt: x.SubmittedAt,
	}); err != nil {
		// 版本落库是尽力而为：判题主链路不允许被它阻塞。
		s.log.Warn("record submit version failed", "submissionId", x.ID, "err", err)
	}
}
