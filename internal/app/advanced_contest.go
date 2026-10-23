package app

import (
	"errors"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

const (
	maxNoticeRunes         = 8000
	maxBroadcastTitleRunes = 200
	maxBroadcastBodyRunes  = 8000
	maxExtensionMinutes    = 7 * 24 * 60
)

// SetIPBindingEnabled 开启/关闭考场 IP 强绑定。
func (s *Service) SetIPBindingEnabled(cid string, enabled bool) (domain.Contest, error) {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		c.BindIPEnabled = enabled
		if c.BoundIPs == nil {
			c.BoundIPs = make(map[string]string)
		}
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}

// BindUserIP 手动为指定考生绑定机位 IP。
func (s *Service) BindUserIP(cid, uid, ip string) error {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return errors.New("invalid IP address")
	}
	ip = parsed.String()
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if c.BoundIPs == nil {
			c.BoundIPs = make(map[string]string)
		}
		c.BoundIPs[uid] = strings.TrimSpace(ip)
		return tx.UpdateContest(c)
	})
	if err != nil {
		return err
	}
	s.emit("contest.ip_bound", cid, map[string]string{"userId": uid, "ip": ip})
	return nil
}

// UnbindUserIP 教师端为因死机/硬件故障换机的单名考生一键解绑机位 IP。
func (s *Service) UnbindUserIP(cid, uid string) error {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if c.BoundIPs != nil {
			delete(c.BoundIPs, uid)
		}
		return tx.UpdateContest(c)
	})
	if err != nil {
		return err
	}
	s.emit("contest.ip_unbound", cid, map[string]string{"userId": uid})
	return nil
}

// UnbindAllIPs 教师端一键重置全考场所有考生的机位 IP 绑定。
func (s *Service) UnbindAllIPs(cid string) error {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		c.BoundIPs = make(map[string]string)
		return tx.UpdateContest(c)
	})
	if err != nil {
		return err
	}
	s.emit("contest.ip_unbound_all", cid, map[string]string{"contestId": cid})
	return nil
}

// GetBoundIPs 获取当前考场的 IP 绑定列表及开关状态。
func (s *Service) GetBoundIPs(cid string) (map[string]string, bool, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return nil, false, err
	}
	if c.BoundIPs == nil {
		return make(map[string]string), c.BindIPEnabled, nil
	}
	return c.BoundIPs, c.BindIPEnabled, nil
}

// SetContestWindow 设定严格考试时间窗口（支持计划起止时间与硬切断）。
func (s *Service) SetContestWindow(cid string, windowEnabled bool, startTime, endTime *time.Time, durationMinutes int) (domain.Contest, error) {
	if err := domain.ValidateContestWindow(windowEnabled, startTime, endTime, durationMinutes, s.clock.Now()); err != nil {
		return domain.Contest{}, err
	}
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		c.WindowEnabled = windowEnabled
		c.StartTime = startTime
		c.EndTime = endTime
		c.DurationMinutes = durationMinutes
		if c.Extensions == nil {
			c.Extensions = make(map[string]int)
		}
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.window_updated", cid, c)
	return c, nil
}

// SetUserExtension 为发生故障换机的个别考生精准加时（Extension 分钟数）。
func (s *Service) SetUserExtension(cid, uid string, minutes int) (domain.Contest, error) {
	if minutes < 0 {
		return domain.Contest{}, errors.New("extension minutes cannot be negative")
	}
	if minutes > maxExtensionMinutes {
		return domain.Contest{}, errors.New("extension minutes exceed the one-week cap")
	}
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if _, err := tx.GetUser(uid); err != nil {
			return errors.New("user not found")
		}
		if c.Extensions == nil {
			c.Extensions = make(map[string]int)
		}
		c.Extensions[uid] = minutes
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.extension_updated", cid, map[string]any{"userId": uid, "minutes": minutes})
	return c, nil
}

// RemoveUserExtension 移除考生的加时。
func (s *Service) RemoveUserExtension(cid, uid string) (domain.Contest, error) {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if c.Extensions != nil {
			delete(c.Extensions, uid)
		}
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.extension_updated", cid, map[string]any{"userId": uid, "minutes": 0})
	return c, nil
}

// UpdateNotice 更新考场置顶通告（全员可见，支持实时下发更正）。
func (s *Service) UpdateNotice(cid, notice string) (domain.Contest, error) {
	notice = strings.TrimSpace(notice)
	if utf8.RuneCountInString(notice) > maxNoticeRunes {
		return domain.Contest{}, errors.New("notice is too long")
	}
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		c.Notice = notice
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.notice", cid, map[string]string{"notice": c.Notice})
	return c, nil
}

// SendBroadcastMessage 下发考场突发广播或定向通知。
func (s *Service) SendBroadcastMessage(cid, senderName, targetUserID, title, content, level string) (domain.BroadcastMessage, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" && content == "" {
		return domain.BroadcastMessage{}, errors.New("title or content is required")
	}
	if level == "" {
		level = "info"
	}
	switch level {
	case "info", "warning", "urgent":
	default:
		return domain.BroadcastMessage{}, errors.New("broadcast level must be info, warning, or urgent")
	}
	if senderName == "" {
		senderName = "监考老师"
	}
	if utf8.RuneCountInString(title) > maxBroadcastTitleRunes {
		return domain.BroadcastMessage{}, errors.New("broadcast title is too long")
	}
	if utf8.RuneCountInString(content) > maxBroadcastBodyRunes {
		return domain.BroadcastMessage{}, errors.New("broadcast content is too long")
	}
	targetUserID = strings.TrimSpace(targetUserID)

	msg := domain.BroadcastMessage{
		ID:           s.store.NextID("msg"),
		ContestID:    cid,
		SenderName:   senderName,
		TargetUserID: targetUserID,
		Title:        title,
		Content:      content,
		Level:        level,
		CreatedAt:    s.clock.Now(),
	}

	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.GetContest(cid)
		if err != nil {
			return err
		}
		if targetUserID != "" && targetUserID != "all" {
			if _, err := tx.GetUser(targetUserID); err != nil {
				return errors.New("broadcast target user not found")
			}
		}
		c.Broadcasts = append(c.Broadcasts, msg)
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.BroadcastMessage{}, err
	}

	s.emit("contest.broadcast", cid, msg)
	return msg, nil
}

// ListBroadcastMessages 获取考生可查看的广播消息列表（全场消息 + 属于该考生的定向消息）。
func (s *Service) ListBroadcastMessages(cid, userID string) ([]domain.BroadcastMessage, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return nil, err
	}
	if len(c.Broadcasts) == 0 {
		return []domain.BroadcastMessage{}, nil
	}

	res := make([]domain.BroadcastMessage, 0, len(c.Broadcasts))
	for _, m := range c.Broadcasts {
		if m.TargetUserID == "" || m.TargetUserID == "all" || m.TargetUserID == userID {
			res = append(res, m)
		}
	}
	return res, nil
}

// SetZipPassword rotates the exam unlock password. Only a salted Argon2id
// verifier is persisted; clearing the value disables the paper gate and
// invalidates all outstanding grants.
func (s *Service) SetZipPassword(cid, password string) (domain.Contest, error) {
	password = strings.TrimSpace(password)
	var verifier string
	if password != "" {
		var err error
		verifier, err = auth.HashZipPassword(password)
		if err != nil {
			return domain.Contest{}, err
		}
	}
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		c.ZipPasswordHash = verifier
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}

// VerifyZipPassword checks an unlock password without ever persisting or
// returning the plaintext value.
func (s *Service) VerifyZipPassword(cid, password string) (bool, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return false, err
	}
	if c.ZipPasswordHash == "" {
		return true, nil // 未设密码直接放行
	}
	return auth.VerifyZipPassword(c.ZipPasswordHash, password)
}
