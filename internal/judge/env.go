package judge

import (
	"os"
	"strings"
)

// scrubEnv 最小环境：保留系统启动必需，其余全部剥离。
func scrubEnv() []string {
	keep := map[string]bool{"SYSTEMROOT": true, "PATH": true, "TMP": true, "TEMP": true, "SYSTEMDRIVE": true}
	var out []string
	for _, kv := range os.Environ() {
		if i := strings.Index(kv, "="); i > 0 && keep[strings.ToUpper(kv[:i])] {
			out = append(out, kv)
		}
	}
	return out
}
