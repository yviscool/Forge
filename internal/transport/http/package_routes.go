package httpapi

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

// importProblemPackageRoute POST /contests/{cid}/problems/import-package
// 接收主流题包 ZIP（Polygon/Hydro/QDUOJ/FPS），自动识别并导入题目与测试数据。
func (h *Server) importProblemPackageRoute(w http.ResponseWriter, r *http.Request, cid string) {
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return
	}

	var data []byte
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			h.err(w, r, 400, fmt.Errorf("parse multipart form: %w", err))
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			h.err(w, r, 400, errors.New("form field 'file' is required"))
			return
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, 64<<20))
		if err != nil {
			h.err(w, r, 400, fmt.Errorf("read file: %w", err))
			return
		}
	} else {
		var err error
		data, err = io.ReadAll(io.LimitReader(r.Body, 64<<20))
		if err != nil {
			h.err(w, r, 400, fmt.Errorf("read request body: %w", err))
			return
		}
	}

	if len(data) == 0 {
		h.err(w, r, 400, errors.New("empty package data"))
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		h.err(w, r, 400, fmt.Errorf("invalid zip archive: %w", err))
		return
	}

	created, err := h.svc.ImportProblemPackage(cid, zr, h.Files)
	if err != nil {
		h.err(w, r, 400, err)
		return
	}

	writeOut(w, created, http.StatusCreated)
}

// exportNOIPackageRoute GET /contests/{cid}/export-package
// 将当前比赛的所有试题与测试数据打包为符合 NOI/CSP 标准规范的 ZIP 离线题包。
func (h *Server) exportNOIPackageRoute(w http.ResponseWriter, r *http.Request, cid string) {
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return
	}

	zipBytes, err := h.svc.ExportNOIContestPackage(cid, h.Files)
	if err != nil {
		h.err(w, r, 500, err)
		return
	}

	fileName := fmt.Sprintf("contest-%s-noi.zip", sanitizeCollectArchiveName(cid))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	_, _ = w.Write(zipBytes)
}
