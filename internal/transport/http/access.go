package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
)

const zipCookieName = "forge_exam_unlock"

// requirePaperAccess adds the exam-package gate on top of contest eligibility.
// Staff bypass the password as operational owners; students must present a
// signed grant tied to both their account and this contest. A client-side
// "unlocked" flag is presentation cache only.
func (h *Server) requirePaperAccess(w http.ResponseWriter, r *http.Request, cid string) (*http.Request, domain.Contest, bool) {
	r2, c, ok := h.requireContestAccess(w, r, cid)
	if !ok {
		return r, domain.Contest{}, false
	}
	if h.paperUnlocked(r2, c) {
		c.ZipUnlocked = !isTeacher(r2) && c.ZipPasswordConfigured()
		return r2, c, true
	}
	WriteError(w, r, http.StatusForbidden, "exam_locked", "exam package is locked")
	return r, domain.Contest{}, false
}

// paperUnlocked is the non-rejecting form of the paper gate. Contest detail
// must remain readable while locked so the student can discover the lock and
// submit the password, but problems/PDF/submissions are still gated.
func (h *Server) paperUnlocked(r *http.Request, c domain.Contest) bool {
	if isTeacher(r) || !c.ZipPasswordConfigured() {
		return true
	}
	sess, ok := sessionOf(r)
	if !ok {
		return false
	}
	cookie, err := r.Cookie(zipCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false
	}
	_, err = h.auth.ValidateZipGrant(
		cookie.Value, sess.UserID, c.ID, h.auth.ZipPasswordBinding(c.ZipPasswordHash),
	)
	return err == nil
}

// issueZipGrant writes the HttpOnly unlock cookie and returns its expiry.
func (h *Server) issueZipGrant(w http.ResponseWriter, r *http.Request, c domain.Contest) (string, error) {
	sess, ok := sessionOf(r)
	if !ok {
		return "", errors.New("login required")
	}
	ticket, expires, err := h.auth.IssueZipGrant(
		sess.UserID, c.ID, sess.Token, h.auth.ZipPasswordBinding(c.ZipPasswordHash), auth.ZipUnlockTTL,
	)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     zipCookieName,
		Value:    ticket,
		Path:     "/api/v1",
		MaxAge:   int(auth.ZipUnlockTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
	return expires.Format(time.RFC3339), nil
}

func (h *Server) clearZipCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     zipCookieName,
		Value:    "",
		Path:     "/api/v1",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
}

// requireContestAccess authenticates the request and applies the shared
// student eligibility policy. Staff can access every contest.
func (h *Server) requireContestAccess(w http.ResponseWriter, r *http.Request, cid string) (*http.Request, domain.Contest, bool) {
	r2, ok := h.requireLogin(w, r)
	if !ok {
		return r, domain.Contest{}, false
	}
	c, err := h.svc.GetContest(cid)
	if err != nil {
		h.err(w, r, 404, err)
		return r, domain.Contest{}, false
	}
	sess, _ := sessionOf(r2)
	u, err := h.svc.Store().GetUser(sess.UserID)
	if err != nil {
		WriteError(w, r, 401, "unauthorized", "session user no longer exists")
		return r, domain.Contest{}, false
	}
	if !domain.CanAccessContest(c, u, isTeacher(r2)) {
		WriteError(w, r, 403, "forbidden", "contest access denied")
		return r, domain.Contest{}, false
	}
	return r2, c, true
}

// visibleContests applies contest filtering for the authenticated session.
func (h *Server) visibleContests(r *http.Request) ([]domain.Contest, bool) {
	sess, ok := sessionOf(r)
	if !ok {
		return nil, false
	}
	u, err := h.svc.Store().GetUser(sess.UserID)
	if err != nil {
		return nil, false
	}
	staff := isTeacher(r)
	out := make([]domain.Contest, 0)
	for _, c := range h.svc.ListContests() {
		if !domain.CanAccessContest(c, u, staff) {
			continue
		}
		if !staff {
			c = redactContest(c, sess.UserID)
		}
		out = append(out, c)
	}
	return out, true
}

// redactContest removes staff-only operational data while preserving the
// fields students need for navigation and timing.
func redactContest(c domain.Contest, userID string) domain.Contest {
	c.ZipProtected = c.ZipPasswordConfigured()
	c.ZipPasswordHash = ""
	c.BoundIPs = nil
	c.Broadcasts = nil
	if minutes, ok := c.Extensions[userID]; ok {
		c.Extensions = map[string]int{userID: minutes}
	} else {
		c.Extensions = nil
	}
	return c
}

// eventForSession applies the same visibility rules to realtime payloads that
// the request handlers apply to REST responses. SSE is a shared contest feed,
// so a raw domain object here would bypass the REST redaction boundary.
func (h *Server) eventForSession(e domain.Event, sess domain.Session) (domain.Event, bool) {
	if sess.Role == domain.RoleTeacher || sess.Role == domain.RoleAdmin {
		return e, true
	}

	switch e.Type {
	case "contest.created", "contest.started", "contest.updated", "contest.finished",
		"contest.window_updated":
		if c, ok := e.Data.(domain.Contest); ok {
			redacted := redactContest(c, sess.UserID)
			if c.ZipPasswordConfigured() {
				// Without per-subscriber unlock state in the SSE transport,
				// withhold protected contest metadata entirely rather than
				// leaking existence or structure.
				return e, false
			}
			e.Data = redacted
		}
	case "contest.notice":
		// Notice payloads only contain the public notice text.
	case "contest.extension_updated":
		if m, ok := e.Data.(map[string]any); ok {
			if uid, _ := m["userId"].(string); uid != "" && uid != sess.UserID {
				return e, false
			}
		}
	case "contest.ip_bound", "contest.ip_unbound", "contest.ip_unbound_all":
		// IP bindings are staff-only operational state.
		return e, false
	case "contest.broadcast":
		if m, ok := e.Data.(domain.BroadcastMessage); ok {
			if m.TargetUserID != "" && m.TargetUserID != "all" && m.TargetUserID != sess.UserID {
				return e, false
			}
		}
	case "submission.created", "submission.judged":
		if s, ok := e.Data.(domain.Submission); ok {
			if s.UserID != sess.UserID {
				// Peer attempts stay out of the candidate event stream entirely.
				// Ranking updates still arrive through their own redacted event.
				return e, false
			}
			// NOI 全真模拟：赛中连本人收到的判定事件也只保留「已提交」形态，
			// 判定与分点在比赛结束后的 REST 视图统一公布。
			if c, err := h.svc.GetContest(s.ContestID); err == nil &&
				!domain.StudentResultsVisible(c.Mode, domain.Status(c.Status)) {
				e.Data = domain.MaskSubmissionForStudent(s)
			}
		}
	case "ranking.updated":
		// NOI 全真模拟：榜单更新不在赛中对选手广播（快照接口同样对学生关闭）。
		if c, err := h.svc.GetContest(e.ContestID); err == nil &&
			!domain.StudentResultsVisible(c.Mode, domain.Status(c.Status)) {
			return e, false
		}
	case "problem.created", "problem.updated":
		if p, ok := e.Data.(domain.Problem); ok {
			if c, err := h.svc.GetContest(p.ContestID); err == nil &&
				c.ZipPasswordConfigured() {
				return e, false
			}
			e.Data = redactProblem(p)
		}
	case "problems.reordered":
		if ps, ok := e.Data.([]domain.Problem); ok {
			if len(ps) > 0 {
				if c, err := h.svc.GetContest(ps[0].ContestID); err == nil &&
					c.ZipPasswordConfigured() {
					return e, false
				}
			}
			out := make([]domain.Problem, len(ps))
			for i, p := range ps {
				out[i] = redactProblem(p)
			}
			e.Data = out
		}
	}
	return e, true
}

// redactProblem removes judge inputs and answer material from student views.
// Choice stems/options remain visible so the exam can be rendered.
func redactProblem(p domain.Problem) domain.Problem {
	p.TestCases = nil
	p.CheckerCode = ""
	p.InteractorCode = ""
	p.ChoiceKeyJSON = ""
	if len(p.ChoiceQuestions) > 0 {
		questions := make([]domain.ChoiceQuestion, len(p.ChoiceQuestions))
		for i, q := range p.ChoiceQuestions {
			q.Answer = nil
			q.Explanation = ""
			questions[i] = q
		}
		p.ChoiceQuestions = questions
	}
	return p
}
