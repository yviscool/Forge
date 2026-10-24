package app

import (
	"errors"

	"github.com/yviscool/forge/internal/domain"
)

// CollectSubmissions 汇总一场比赛的收题包：名单、准考证目录、每人每题最后
// 一次提交。比赛永远不拥有号——收题时用（赛季, 组别）反查准考证，普通
// 在线赛（未声明组别）退回登录名目录。
func (s *Service) CollectSubmissions(cid string) (domain.CollectPackage, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.CollectPackage{}, err
	}
	problems, err := s.store.ListProblems(cid)
	if err != nil {
		return domain.CollectPackage{}, err
	}
	users, err := s.store.ListUsers()
	if err != nil {
		return domain.CollectPackage{}, err
	}
	groups, err := s.store.ListGroups()
	if err != nil {
		return domain.CollectPackage{}, err
	}
	tickets, err := s.store.ListTickets()
	if err != nil {
		return domain.CollectPackage{}, err
	}
	subs, err := s.store.ListSubmissions(cid)
	if err != nil {
		return domain.CollectPackage{}, err
	}
	if len(c.ProblemIDs) == 0 && len(problems) == 0 {
		return domain.CollectPackage{}, errors.New("contest has no problems to collect")
	}
	return domain.ComputeCollectLayout(domain.CollectSource{
		Contest:     c,
		Groups:      groups,
		Users:       users,
		Problems:    problems,
		Tickets:     tickets,
		Submissions: subs,
	}), nil
}
