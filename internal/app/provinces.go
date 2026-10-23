package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// ProvinceView 省份管理视图：内置行政区划 + 运营状态 + 实时用户数。
type ProvinceView struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Sort     int    `json:"sort"`
	Enabled  bool   `json:"enabled"`
	Users    int    `json:"users"`
	HasUsers bool   `json:"hasUsers"`
}

// CreateAccount 创建账号（CreateUser 家族的完整形态）：省份取值
// 必须是内置省份且处于启用状态——准考证号链路不允许悬空引用。
func (s *Service) CreateAccount(name, username, role, province string) (domain.User, error) {
	province, err := s.normalizeEnabledProvince(province)
	if err != nil {
		return domain.User{}, err
	}
	return s.createUserRecord(name, username, role, province, "")
}

func (s *Service) normalizeEnabledProvince(province string) (string, error) {
	// 注意：必须在事务外读取状态表。memory 事务实现全程持有写锁，
	// 闭包内再读 store 会自锁；sqlite 侧同理属于多余的事务内查询。
	states, err := s.store.ProvinceStates()
	if err != nil {
		return "", err
	}
	return normalizeProvinceWithStates(province, states)
}

// normalizeProvinceWithStates 纯函数校验：归一化省份取值并检查启用状态，
// 可安全地在事务闭包内使用（状态快照由调用方在事务外取得）。
func normalizeProvinceWithStates(province string, states map[string]bool) (string, error) {
	ref, err := domain.NormalizeProvinceRef(province)
	if err != nil {
		return "", err
	}
	if ref == "" {
		return "", nil
	}
	enabled, ok := states[ref]
	if !ok {
		enabled = true
	}
	if !enabled {
		return "", fmt.Errorf("province %s is disabled", ref)
	}
	return ref, nil
}

// UpdateUserInput 编辑账号：角色留空表示不变更；省份仅在取值变化时校验启用状态，
// 因此已经挂到某省的存量用户不会因为该省事后停用而无法改名字。
type UpdateUserInput struct {
	Name     string
	Username string
	Role     string
	Province string
}

// UpdateUser 编辑账号（姓名/登录名/角色/省份）。权限走角色矩阵：
// 教师只能编辑学生，角色升降只能由管理员操作。
func (s *Service) UpdateUser(actorRole, id string, in UpdateUserInput) (domain.User, error) {
	// 省份状态快照在事务外取得（见 normalizeProvinceWithStates 注释）。
	states, err := s.store.ProvinceStates()
	if err != nil {
		return domain.User{}, err
	}
	var updated domain.User
	err = s.withTx(func(tx ports.Store) error {
		target, err := tx.GetUser(id)
		if err != nil {
			return err
		}
		if !domain.CanManageUser(actorRole, target.Role) {
			return fmt.Errorf("forbidden: %s cannot manage %s accounts", actorRole, target.Role)
		}

		name := strings.TrimSpace(in.Name)
		if name == "" {
			return errors.New("name is required")
		}
		username := strings.TrimSpace(in.Username)
		if username == "" {
			return errors.New("username is required")
		}

		role := target.Role
		if in.Role != "" && !strings.EqualFold(in.Role, target.Role) {
			requested := domain.NormalizeRole(in.Role)
			if !domain.ValidRole(requested) {
				return fmt.Errorf("unknown role %q", in.Role)
			}
			if !domain.CanAssignRole(actorRole, requested) {
				return fmt.Errorf("forbidden: %s cannot assign role %s", actorRole, requested)
			}
			role = requested
		}

		province := target.Province
		if !strings.EqualFold(strings.TrimSpace(in.Province), target.Province) {
			normalized, err := normalizeProvinceWithStates(in.Province, states)
			if err != nil {
				return err
			}
			province = normalized
		}

		// 用户名冲突检查排除自身；sqlite 的唯一索引按 lower(trim(username)) 判定。
		users, err := tx.ListUsers()
		if err != nil {
			return err
		}
		for _, u := range users {
			if u.ID != target.ID && strings.EqualFold(u.Username, username) {
				return fmt.Errorf("username %q already exists", username)
			}
		}

		target.Name = name
		target.Username = username
		target.Role = role
		target.Province = province
		if err := tx.UpdateUser(target); err != nil {
			return err
		}
		updated = target
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return updated, nil
}

// BatchSetProvince 批量设置省份：事务内全量成功或整体失败，
// 目标账号必须全部处于操作者可管理范围（教师 → 仅学生）。
func (s *Service) BatchSetProvince(actorRole string, userIDs []string, province string) (int, error) {
	if len(userIDs) == 0 {
		return 0, errors.New("no users selected")
	}
	normalized, err := s.normalizeEnabledProvince(province)
	if err != nil {
		return 0, err
	}
	if normalized == "" {
		return 0, errors.New("province is required")
	}
	updated := 0
	err = s.withTx(func(tx ports.Store) error {
		updated = 0
		for _, id := range userIDs {
			target, err := tx.GetUser(id)
			if err != nil {
				return fmt.Errorf("user %s: %w", id, err)
			}
			if !domain.CanManageUser(actorRole, target.Role) {
				return fmt.Errorf("forbidden: %s cannot manage %s accounts", actorRole, target.Role)
			}
			if target.Province == normalized {
				continue
			}
			target.Province = normalized
			if err := tx.UpdateUser(target); err != nil {
				return fmt.Errorf("user %s: %w", id, err)
			}
			updated++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return updated, nil
}

// CSPTicketRequest CSP 准考证号分配请求：CreateCount>0 时批量创建考生账号；
// 否则把号段分配给 UserIDs 指定的既有账号（教师只能给可管理的学生发号）。
type CSPTicketRequest struct {
	Province     string
	Group        string
	NumberLength int
	CreateCount  int
	UserIDs      []string
}

// CSPTicketAssignment 一次发号结果：账号与最终准考证号一一对应；
// Created 标记该账号是否为本次新建（统一初始密码只作用于新建账号）。
type CSPTicketAssignment struct {
	User    domain.User `json:"user"`
	Ticket  string      `json:"ticket"`
	Created bool        `json:"created"`
}

// AssignCSPTickets 按省份+组别从当前最大序号之后连续发号，
// 号码长度只影响补零宽度（超宽自然升位），同一 (省份, 组别) 永不重号。
// 事务内完成取号与写库；sqlite 侧另有 exam_ticket 唯一索引兜底。
func (s *Service) AssignCSPTickets(actorRole string, req CSPTicketRequest) ([]CSPTicketAssignment, error) {
	province, err := s.normalizeEnabledProvince(req.Province)
	if err != nil {
		return nil, err
	}
	if province == "" {
		return nil, errors.New("province is required")
	}
	group := domain.NormalizeTicketGroup(req.Group)
	if !domain.ValidTicketGroup(group) {
		return nil, fmt.Errorf("ticket group must be J or S, got %q", req.Group)
	}
	digits := req.NumberLength
	if digits == 0 {
		digits = domain.DefaultTicketDigits
	}
	if digits < domain.MinTicketDigits || digits > domain.MaxTicketDigits {
		return nil, fmt.Errorf("number length must be %d-%d digits, got %d",
			domain.MinTicketDigits, domain.MaxTicketDigits, digits)
	}
	if req.CreateCount < 0 || len(req.UserIDs) < 0 {
		return nil, errors.New("invalid ticket request")
	}
	if (req.CreateCount > 0) == (len(req.UserIDs) > 0) {
		return nil, errors.New("specify either createCount or userIds, not both")
	}
	if req.CreateCount > 500 || len(req.UserIDs) > 500 {
		return nil, errors.New("batch ticket assignment is limited to 500")
	}
	if !domain.CanAssignRole(actorRole, domain.RoleStudent) && actorRole != domain.RoleAdmin {
		return nil, fmt.Errorf("forbidden: %s cannot manage candidates", actorRole)
	}

	var out []CSPTicketAssignment
	err = s.withTx(func(tx ports.Store) error {
		out = nil
		users, err := tx.ListUsers()
		if err != nil {
			return err
		}
		next := 1
		for _, u := range users {
			if n, ok := domain.ParseCSPTicketNumber(u.ExamTicket, province, group); ok && n >= next {
				next = n + 1
			}
		}

		issue := func() string {
			ticket := domain.FormatCSPTicket(province, group, next, digits)
			next++
			return ticket
		}

		if req.CreateCount > 0 {
			for i := 0; i < req.CreateCount; i++ {
				ticket := issue()
				created, err := s.createStudentWithTicket(tx, province, ticket)
				if err != nil {
					return err
				}
				out = append(out, CSPTicketAssignment{User: created, Ticket: ticket, Created: true})
			}
			return nil
		}

		byID := make(map[string]domain.User, len(users))
		for _, u := range users {
			byID[u.ID] = u
		}
		for _, id := range req.UserIDs {
			target, ok := byID[id]
			if !ok {
				return fmt.Errorf("user %s not found", id)
			}
			if !domain.CanManageUser(actorRole, target.Role) {
				return fmt.Errorf("forbidden: %s cannot manage %s accounts", actorRole, target.Role)
			}
			if target.Province != "" && target.Province != province {
				return fmt.Errorf("user %s already belongs to province %s", target.Username, target.Province)
			}
			if target.ExamTicket != "" {
				return fmt.Errorf("user %s already has ticket %s", target.Username, target.ExamTicket)
			}
			ticket := issue()
			target.Province = province
			target.ExamTicket = ticket
			if err := tx.UpdateUser(target); err != nil {
				return err
			}
			out = append(out, CSPTicketAssignment{User: target, Ticket: ticket, Created: false})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// createStudentWithTicket 在事务内创建一名带准考证号的考生账号。
// 登录名取准考证号的紧凑小写形式；历史遗留同名时自动加后缀，准考证号本身不受影响。
func (s *Service) createStudentWithTicket(tx ports.Store, province, ticket string) (domain.User, error) {
	compact := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(ticket))
	name := "考生" + ticket
	users, err := tx.ListUsers()
	if err != nil {
		return domain.User{}, err
	}
	username := compact
	for i := 2; ; i++ {
		taken := false
		for _, u := range users {
			if strings.EqualFold(u.Username, username) {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		username = fmt.Sprintf("%s_%d", compact, i)
	}
	return tx.CreateUser(domain.User{
		ID:         tx.NextID("usr"),
		Username:   username,
		Name:       name,
		Role:       domain.RoleStudent,
		Province:   province,
		ExamTicket: ticket,
		Groups:     []string{},
		CreatedAt:  s.clock.Now(),
	})
}

// ListProvinces 返回内置省份 + 启用状态 + 各省用户数（含教师/管理员以外全部账号）。
func (s *Service) ListProvinces() ([]ProvinceView, error) {
	states, err := s.store.ProvinceStates()
	if err != nil {
		return nil, err
	}
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, u := range users {
		if u.Province != "" {
			counts[u.Province]++
		}
	}
	views := make([]ProvinceView, 0, len(domain.Provinces()))
	for _, p := range domain.Provinces() {
		enabled, ok := states[p.Code]
		if !ok {
			enabled = true
		}
		views = append(views, ProvinceView{
			Code: p.Code, Name: p.Name, Sort: p.Sort,
			Enabled: enabled, Users: counts[p.Code], HasUsers: counts[p.Code] > 0,
		})
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Sort < views[j].Sort })
	return views, nil
}

// SetProvinceEnabled 启用/停用省份。这是考务策略配置，仅管理员可操作；
// 停用不影响已关联用户，只阻止新分配。
func (s *Service) SetProvinceEnabled(actorRole, code string, enabled bool) error {
	if actorRole != domain.RoleAdmin {
		return fmt.Errorf("forbidden: only administrators can configure provinces")
	}
	canonical, ok := domain.ProvinceByCode(code)
	if !ok {
		return fmt.Errorf("unknown province %q", code)
	}
	return s.store.SetProvinceState(canonical.Code, enabled)
}
