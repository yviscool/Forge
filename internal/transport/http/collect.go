package httpapi

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// collectRoute GET /contests/{cid}/collect：导出 CSP 收题包。
//
//   - 默认返回 zip：<准考证号>/<题面英文名>/<英文名>.<ext> + manifest.csv；
//     普通在线赛（未声明 ticketGroup）目录退回登录名。
//   - ?format=json 返回收题清单与告警，供教师台在下载前预览缺号/白卷。
func (h *Server) collectRoute(w http.ResponseWriter, r *http.Request, cid string) {
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return
	}
	pkg, err := h.svc.CollectSubmissions(cid)
	if err != nil {
		h.err(w, r, 404, err)
		return
	}
	if r.URL.Query().Get("format") == "json" {
		writeOut(w, pkg, 200)
		return
	}

	name := "collect-" + sanitizeCollectArchiveName(cid) + ".zip"
	if pkg.Season != "" {
		name = fmt.Sprintf("collect-%s-%s.zip",
			sanitizeCollectArchiveName(pkg.Season), sanitizeCollectArchiveName(pkg.Group))
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	if err := writeCollectZip(w, pkg); err != nil {
		h.log.Error("collect zip failed", "contestId", cid, "err", err)
	}
}

// writeCollectZip 把收题清单落成 zip 流。manifest.csv 放在根目录：目录名
// 与文件名都保持 CSP 口径，评卷工具只看 <准考证号>/<英文名>/<英文名>.cpp。
func writeCollectZip(w http.ResponseWriter, pkg domain.CollectPackage) error {
	zw := zip.NewWriter(w)
	for _, e := range pkg.Entries {
		folder := strings.Trim(domain.SanitizeCollectPath(e.Folder), ".")
		name := strings.Trim(domain.SanitizeCollectPath(e.FileName), ".")
		if folder == "" || name == "" {
			continue
		}
		f, err := zw.Create(folder + "/" + name)
		if err != nil {
			return err
		}
		if _, err := f.Write([]byte(e.Code)); err != nil {
			return err
		}
	}
	if err := writeCollectManifest(zw, pkg); err != nil {
		return err
	}
	return zw.Close()
}

// writeCollectManifest 收题清单 CSV：一份对账表，教师拿到包后能逐行核对
// 目录名、准考证号、题号、最后一次提交的判题结果。
func writeCollectManifest(zw *zip.Writer, pkg domain.CollectPackage) error {
	f, err := zw.Create("manifest.csv")
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("folder,ticket,name,username,problem,slug,file,language,verdict,score,submittedAt\r\n")
	for _, e := range pkg.Entries {
		row := []string{
			e.Folder, e.Ticket, e.UserName, e.Username,
			e.ProblemCode, e.Slug, e.FileName, e.Language,
			e.Verdict, fmt.Sprint(e.Score), e.SubmittedAt.UTC().Format(time.RFC3339),
		}
		for i, cell := range row {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(csvText(cell))
		}
		b.WriteString("\r\n")
	}
	_, err = f.Write([]byte(b.String()))
	return err
}

// sanitizeCollectArchiveName 下载文件名里只留安全字符。
func sanitizeCollectArchiveName(s string) string {
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", " ", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-",
	)
	return strings.TrimSpace(repl.Replace(s))
}
