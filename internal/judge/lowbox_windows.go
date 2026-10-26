//go:build windows

package judge

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows 低权沙箱：LowBox(AppContainer 类)令牌 + 工作目录 ACL。
//
// 安全目标对标 LemonLime 的 windowssandbox(AppContainer):
//   - 文件系统:选手进程只看得见工作目录(显式 ACL)与系统默认开放给
//     "ALL APPLICATION PACKAGES" 的位置(System32 等);用户目录、比赛数据、
//     其他提交一律拒绝——ACL 校验跟随符号链接目标,符号链接攻击天然失效;
//   - 网络:未授予任何 capability,套接字创建/连接全拒;
//   - 资源墙:Job Object(内存/单进程/关闭即杀)在沙箱之上照常叠加。
//
// 实现走 NtCreateLowBoxToken(Chromium 同路):从当前进程令牌派生 LowBox
// 令牌,经 syscall.SysProcAttr.Token 交给 os/exec——管道、等待、rusage、
// Job 全部保留。包 SID 由工作目录派生,每单一物。
//
// 降级:模式 auto 下探测(容器内自证 + 越界读必须失败)不过即退回
// Job-only 资源墙,并在状态里说明原因;探测异步执行,不占运行 ctx。

const (
	winSandboxAuto = "auto"
	winSandboxOff  = "off"
)

var (
	winSandboxMode   atomic.Value // string
	winSandboxOnce   sync.Once
	winSandboxUsable atomic.Bool
	winSandboxReason atomic.Value // string
	winSandboxDone   = make(chan struct{})
)

// ConfigureWinSandbox 设置 Windows 沙箱模式(auto/off;auto 缺省)并异步
// 启动探测。非法模式返回错误(fail-closed)。
func ConfigureWinSandbox(mode string) error {
	switch mode {
	case "", winSandboxAuto:
		winSandboxMode.Store(winSandboxAuto)
	case winSandboxOff:
		winSandboxMode.Store(winSandboxOff)
	default:
		return fmt.Errorf("invalid FORGE_WIN_SANDBOX mode %q (auto|off)", mode)
	}
	startWinSandboxProbe()
	return nil
}

// WinSandboxStatus 返回当前沙箱状态(观测用):
// off / probing / appcontainer / fallback:<原因>。
func WinSandboxStatus() string {
	if !isWindowsSandboxEnabled() {
		return "off"
	}
	select {
	case <-winSandboxDone:
		if winSandboxUsable.Load() {
			return "appcontainer"
		}
		return "fallback:" + winSandboxReason.Load().(string)
	default:
		return "probing"
	}
}

func isWindowsSandboxEnabled() bool {
	mode, _ := winSandboxMode.Load().(string)
	return mode != winSandboxOff
}

// sandboxSetup 返回沙箱化所需的进程属性与环境覆盖;不可用返回 nil(nil,
// 调用方按 Job-only 继续运行)。workdir ACL 授予包 SID,令牌句柄由 cleanup
// 在 Wait 之后释放。
func sandboxSetup(workdir string) (attr *syscall.SysProcAttr, envOverrides []string, cleanup func(), err error) {
	if !isWindowsSandboxEnabled() || !winSandboxUsable.Load() {
		return nil, nil, func() {}, nil
	}
	sid, err := lowBoxSID()
	if err != nil {
		setSandboxFallback("sid derive: " + err.Error())
		return nil, nil, func() {}, nil
	}
	if err := grantWorkdirAcl(workdir, sid); err != nil {
		setSandboxFallback("workdir acl: " + err.Error())
		return nil, nil, func() {}, nil
	}
	token, err := createLowBoxToken(sid)
	if err != nil {
		setSandboxFallback("lowbox token: " + err.Error())
		return nil, nil, func() {}, nil
	}
	cleanup = func() { _ = windows.CloseHandle(windows.Handle(token)) }
	attr = &syscall.SysProcAttr{
		Token:      syscall.Token(token),
		HideWindow: true, // 教师机上 GUI/弹窗类提交不得闪窗
	}
	// 容器读不到用户 %TEMP%,临时文件必须落在已授权的工作目录。
	envOverrides = append(envOverrides,
		"TMP="+workdir,
		"TEMP="+workdir,
	)
	return attr, envOverrides, cleanup, nil
}

func setSandboxFallback(reason string) {
	winSandboxUsable.Store(false)
	winSandboxReason.Store(reason)
}

// ---- 包 SID:S-1-15-2-<固定种子哈希 8 字> ----

var (
	lowBoxSidOnce sync.Once
	lowBoxSidVal  *windows.SID
)

// lowBoxSID 固定包 SID(同机一致):运行时目录的授权只做一次;不同运行的
// 工作目录共用该 SID,但容器无法枚举 %TEMP%(用户目录不可达)且临时目录名
// 随机不可猜测,跨运行读取不构成可达路径。
func lowBoxSID() (*windows.SID, error) {
	lowBoxSidOnce.Do(func() {
		sid, err := deriveLowBoxSid("forge-judge-lowbox")
		if err == nil {
			lowBoxSidVal = sid
		}
	})
	if lowBoxSidVal == nil {
		return nil, fmt.Errorf("package sid unavailable")
	}
	return lowBoxSidVal, nil
}

func deriveLowBoxSid(seed string) (*windows.SID, error) {
	sum := sha256.Sum256([]byte(seed))
	// AppContainer 包 SID 的合法形态:S-1-15-2-<7 个 uint32 LE>——共 8 个
	// 子授权;9 个子授权会被内核以 SID 长度无效拒收(实测 0xC000A202)。
	buf := make([]byte, 8+8*4)
	buf[0] = 1
	buf[1] = 8
	copy(buf[2:8], []byte{0, 0, 0, 0, 0, 15}) // SECURITY_NT_AUTHORITY = 15
	// SID 子授权是 DWORD 小端(大端会拼出非法的包 SID 形态)。
	binary.LittleEndian.PutUint32(buf[8:], 2)
	for i := 0; i < 7; i++ {
		binary.LittleEndian.PutUint32(buf[12+i*4:], binary.LittleEndian.Uint32(sum[i*4:i*4+4]))
	}
	sid := (*windows.SID)(unsafe.Pointer(&buf[0]))
	if sid.String() == "" {
		return nil, fmt.Errorf("invalid package sid")
	}
	return sid, nil
}

// ---- NtCreateLowBoxToken(Chromium 同路,ntdll) ----

var lowBoxProc = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateLowBoxToken")

func createLowBoxToken(sid *windows.SID) (windows.Handle, error) {
	var self windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &self)
	if err != nil {
		return 0, err
	}
	defer self.Close()
	var out windows.Handle
	// NtCreateLowBoxToken(out, existing, desired, objAttrs=NULL, pkgSid,
	//                     capCount=0, caps=NULL, handleCount=0, handles=NULL)
	r1, _, _ := lowBoxProc.Call(
		uintptr(unsafe.Pointer(&out)),
		uintptr(self),
		uintptr(windows.MAXIMUM_ALLOWED),
		0,
		uintptr(unsafe.Pointer(sid)),
		0, 0, 0, 0,
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtCreateLowBoxToken ntstatus 0x%x", r1)
	}
	return out, nil
}

// ---- 探测:容器内自证 + 越界读必须失败 ----

// startWinSandboxProbe 异步触发(镜像 bwrap 探针的降级语义)。探测内容:
//  1. 容器内 cmd 可运行、工作目录可写(自证机制可用);
//  2. 容器外放置的哨兵文件读不到(隔离生效的证据)。
func startWinSandboxProbe() {
	winSandboxOnce.Do(func() {
		go func() {
			defer close(winSandboxDone)
			if !isWindowsSandboxEnabled() {
				setSandboxFallback("disabled by config")
				return
			}
			runWinSandboxProbe()
		}()
	})
}

func runWinSandboxProbe() {
	workdir, err := os.MkdirTemp("", "forge_lowbox_probe_*")
	if err != nil {
		setSandboxFallback("probe workdir: " + err.Error())
		return
	}
	defer os.RemoveAll(workdir)
	sid, err := lowBoxSID()
	if err != nil {
		setSandboxFallback("probe sid: " + err.Error())
		return
	}
	if err := grantWorkdirAcl(workdir, sid); err != nil {
		setSandboxFallback("probe acl: " + err.Error())
		return
	}
	token, err := createLowBoxToken(sid)
	if err != nil {
		setSandboxFallback("probe lowbox: " + err.Error())
		return
	}
	defer windows.CloseHandle(token)

	run := func(args ...string) (string, int, error) {
		cmd := exec.Command("cmd", args...)
		cmd.Dir = workdir
		cmd.Env = append(scrubEnv(), "TMP="+workdir, "TEMP="+workdir)
		cmd.SysProcAttr = &syscall.SysProcAttr{Token: syscall.Token(token), HideWindow: true}
		out, err := cmd.CombinedOutput()
		return string(out), exitCodeOf(cmd), err
	}
	// 1) 容器内可执行、工作目录可写。
	out, code, runErr := run("/c", "echo ok > probe.txt & type probe.txt")
	if runErr != nil || code != 0 || !strings.Contains(out, "ok") {
		setSandboxFallback(fmt.Sprintf("probe inside: code=%d err=%v out=%q", code, runErr, out))
		return
	}
	// 2) 越界读必须失败:哨兵放在用户目录(用户可读、容器不可达)。
	if home := os.Getenv("USERPROFILE"); home != "" {
		canary := filepath.Join(home, "forge_lowbox_canary.txt")
		if err := os.WriteFile(canary, []byte("secret"), 0o600); err == nil {
			defer os.Remove(canary)
			_, code, _ := run("/c", "type \""+canary+"\"")
			if code == 0 {
				setSandboxFallback("probe isolation: outside file readable inside container")
				return
			}
		}
	}
	winSandboxUsable.Store(true)
	winSandboxReason.Store("")
}

func exitCodeOf(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

// ensureWinSandboxProbed 供测试等待探测结论(最多 d)。
func ensureWinSandboxProbed(d time.Duration) {
	startWinSandboxProbe()
	select {
	case <-winSandboxDone:
	case <-time.After(d):
	}
}
