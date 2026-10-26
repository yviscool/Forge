//go:build windows

package judge

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 工作目录/运行时目录 ACL:为包 SID 追加允许 ACE。SetNamedSecurityInfo
// 会把可继承 ACE 自动传播到既有子对象——目录一次授予即覆盖整棵树
// (对标 LemonLime windowssandbox 的 ACL 授权,不做逐文件遍历)。
// ACE 为追加式(grant-only):工作目录是一次性临时目录;运行时目录授予
// 只读+执行,属最小授权的良性变更。

// getNamedSecurityInfoRaw 读原始 DACL(x/sys 类型化返回不带 DACL 访问器)。
// 返回的 DACL 指针存活于描述符内,释放描述符即可(勿单独 free 内部指针)。
var getNamedSecurityInfoRaw = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetNamedSecurityInfoW")

// setEntriesInAclW advapi32 的 SetEntriesInAclW(x/sys 未导出包装):
// DWORD SetEntriesInAclW(ULONG, PEXPLICIT_ACCESS, PACL, PACL*)。
// 第二参用 x/sys 的 windows.EXPLICIT_ACCESS,布局由官方定义保证。
var setEntriesInAclW = windows.NewLazySystemDLL("advapi32.dll").NewProc("SetEntriesInAclW")

// grantAcl 给 path 追加一条针对 sid 的 GRANT ACE(inherit = 目录可继承)。
func grantAcl(path string, sid *windows.SID, access uint32, inherit bool) error {
	utf16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var oldDacl *windows.ACL
	var sd uintptr
	if r1, _, _ := getNamedSecurityInfoRaw.Call(
		uintptr(unsafe.Pointer(utf16Path)),
		uintptr(windows.SE_FILE_OBJECT),
		uintptr(windows.DACL_SECURITY_INFORMATION),
		0, 0,
		uintptr(unsafe.Pointer(&oldDacl)),
		0,
		uintptr(unsafe.Pointer(&sd)),
	); r1 != 0 {
		return fmt.Errorf("GetNamedSecurityInfoW: error %d", r1)
	}
	defer func() {
		if sd != 0 {
			windows.LocalFree(windows.Handle(sd))
		}
	}()

	inheritance := uint32(0)
	if inherit {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	ea := windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.ACCESS_MASK(access),
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
	var newDacl *windows.ACL
	if r1, _, _ := setEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		uintptr(unsafe.Pointer(oldDacl)),
		uintptr(unsafe.Pointer(&newDacl)),
	); r1 != 0 {
		// SetEntriesInAclW 直接返回 DWORD 错误码(GetLastError 不参与)。
		return fmt.Errorf("SetEntriesInAclW: error %d", r1)
	}
	defer func() {
		if newDacl != nil {
			newDaclPtr := uintptr(unsafe.Pointer(newDacl))
			windows.LocalFree(windows.Handle(newDaclPtr))
		}
	}()
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION, nil, nil, newDacl, nil); err != nil {
		return fmt.Errorf("SetNamedSecurityInfo: %w", err)
	}
	return nil
}

// grantWorkdirAcl 工作目录整树读写执行(新文件靠继承自动带上)。
func grantWorkdirAcl(workdir string, sid *windows.SID) error {
	return grantAcl(workdir, sid, 0x1F01FF /* GENERIC_ALL 映射 */, true)
}

// grantRuntimeDirAcl 运行时目录只读+执行(编译器/解释器自身与同层 DLL)。
func grantRuntimeDirAcl(dir string, sid *windows.SID) error {
	// FILE_GENERIC_READ|EXECUTE 的等价位:读数据/属性 + 执行/遍历。
	return grantAcl(dir, sid, 0x1200A9, true)
}

// revokeAcl 摘除 path 上针对 sid 的 ACE(REVOKE_ACCESS,停机回滚)。
func revokeAcl(path string, sid *windows.SID) error {
	utf16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var oldDacl *windows.ACL
	var sd uintptr
	if r1, _, _ := getNamedSecurityInfoRaw.Call(
		uintptr(unsafe.Pointer(utf16Path)),
		uintptr(windows.SE_FILE_OBJECT),
		uintptr(windows.DACL_SECURITY_INFORMATION),
		0, 0,
		uintptr(unsafe.Pointer(&oldDacl)),
		0,
		uintptr(unsafe.Pointer(&sd)),
	); r1 != 0 {
		return fmt.Errorf("GetNamedSecurityInfoW: error %d", r1)
	}
	defer func() {
		if sd != 0 {
			windows.LocalFree(windows.Handle(sd))
		}
	}()
	ea := windows.EXPLICIT_ACCESS{
		AccessMode: windows.REVOKE_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
	var newDacl *windows.ACL
	if r1, _, _ := setEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		uintptr(unsafe.Pointer(oldDacl)),
		uintptr(unsafe.Pointer(&newDacl)),
	); r1 != 0 {
		return fmt.Errorf("SetEntriesInAclW: error %d", r1)
	}
	defer func() {
		if newDacl != nil {
			newDaclPtr := uintptr(unsafe.Pointer(newDacl))
			windows.LocalFree(windows.Handle(newDaclPtr))
		}
	}()
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION, nil, nil, newDacl, nil); err != nil {
		return fmt.Errorf("SetNamedSecurityInfo: %w", err)
	}
	return nil
}

// runtimeDirAclEnabled 运行时目录授权只在沙箱模式下需要。
func runtimeDirAclEnabled() bool { return isWindowsSandboxEnabled() }

var (
	runtimeAclDone    sync.Map     // command -> struct{}(已授权)
	runtimeAclGranted sync.Map     // dir -> struct{}(本进程授予过,停机回滚用)
	runtimeAclErr     atomic.Value // string(最后一次授权失败原因,观测用)
)

// prepareRuntimeDir 编译时同步授予运行时目录(编译器 bin / Python prefix)
// 只读 ACL——稳定 SID 下继承 ACE 会传播到既有子树,按 command 去重。
// 必须同步:异步授权赶不上紧随其后的首次运行(解释型首跑即 RE 的教训)。
func prepareRuntimeDir(lang, command string) {
	if !runtimeDirAclEnabled() {
		return
	}
	if _, done := runtimeAclDone.LoadOrStore(command, struct{}{}); done {
		return
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		runtimeAclErr.Store("lookpath: " + err.Error())
		return
	}
	sid, err := lowBoxSID()
	if err != nil {
		runtimeAclErr.Store("sid: " + err.Error())
		return
	}
	dirs := []string{filepath.Dir(resolved)}
	if lang == "python" || lang == "python3" || lang == "py" {
		// Python 前缀含 Lib/ 标准库;继承 ACE 传播到既有子树。
		prefix := filepath.Dir(filepath.Dir(resolved))
		out, err := exec.Command(resolved, "-I", "-S", "-c", "import sys; print(sys.prefix)").Output()
		if err == nil {
			if p := strings.TrimSpace(string(out)); p != "" && p != prefix {
				dirs = append(dirs, p)
			}
		}
		dirs = append(dirs, prefix)
	}
	for _, dir := range dirs {
		if dir == "" || dir == string(filepath.Separator) {
			continue
		}
		if err := grantRuntimeDirAcl(dir, sid); err != nil {
			runtimeAclErr.Store(dir + ": " + err.Error())
			return
		}
		runtimeAclGranted.Store(dir, struct{}{})
	}
}
