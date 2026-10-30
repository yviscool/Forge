package packageio

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ReadZipEntries 安全读取并索引 ZIP 归档中的普通文件。
// 严格防范 Zip Slip 目录穿越攻击、单文件超限以及解压体积炸弹。
func ReadZipEntries(r *zip.Reader) (map[string][]byte, error) {
	if len(r.File) > MaxZipEntries {
		return nil, fmt.Errorf("archive contains too many files: %d (max %d)", len(r.File), MaxZipEntries)
	}
	var totalSize int64
	files := make(map[string][]byte, len(r.File))

	for _, f := range r.File {
		// 忽略目录条目
		if f.FileInfo().IsDir() {
			continue
		}
		cleanName := filepath.ToSlash(filepath.Clean(f.Name))
		cleanName = strings.TrimPrefix(cleanName, "/")
		if strings.HasPrefix(cleanName, "../") || cleanName == ".." || strings.Contains(cleanName, "/../") {
			return nil, fmt.Errorf("malicious path traversal in archive: %q", f.Name)
		}
		if f.UncompressedSize64 > MaxSingleFileSize {
			return nil, fmt.Errorf("file %q exceeds max single file size limit: %d", f.Name, f.UncompressedSize64)
		}
		totalSize += int64(f.UncompressedSize64)
		if totalSize > MaxTotalPackageSize {
			return nil, fmt.Errorf("total uncompressed size exceeds limit: %d", totalSize)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open entry %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, MaxSingleFileSize+1))
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		if int64(len(data)) > MaxSingleFileSize {
			return nil, fmt.Errorf("entry %s exceeded size limit during read", f.Name)
		}
		files[cleanName] = data
	}

	if len(files) == 0 {
		return nil, errors.New("empty archive")
	}
	return files, nil
}
