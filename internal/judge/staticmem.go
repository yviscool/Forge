package judge

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"os"
)

// StaticMemoryBytes 读取可执行文件的静态内存足迹（字节）：
// PE 按节表 VirtualSize 之和、ELF 按 PT_LOAD p_memsz 之和、
// Mach-O 按 Load 段 vmsize 之和。
//
// 对标 LemonLime watcher 的 calculateStaticMemoryUsage：选手在 BSS 里
// 开出超大全局数组时（OI 经典坑 `int a[256<<20]`），不必等运行撞内存墙，
// 起跑前即可判 MLE，省一次必然失败的运行。
// 解析失败返回错误，调用方按“无法判定”跳过预检，不影响正常评测。
func StaticMemoryBytes(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if exe, err := pe.NewFile(f); err == nil {
		defer exe.Close()
		var total uint64
		for _, sec := range exe.Sections {
			total += uint64(sec.VirtualSize)
		}
		// PE 头部与对齐填充不可忽略，叠加 SizeOfHeaders 口径。
		if exe.OptionalHeader != nil {
			switch oh := exe.OptionalHeader.(type) {
			case *pe.OptionalHeader32:
				total += uint64(oh.SizeOfHeaders)
			case *pe.OptionalHeader64:
				total += uint64(oh.SizeOfHeaders)
			}
		}
		return total, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	if exe, err := elf.NewFile(f); err == nil {
		defer exe.Close()
		var total uint64
		for _, seg := range exe.Progs {
			if seg.Type == elf.PT_LOAD {
				total += seg.Memsz
			}
		}
		return total, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	if exe, err := macho.NewFile(f); err == nil {
		defer exe.Close()
		var total uint64
		for _, load := range exe.Loads {
			if seg, ok := load.(*macho.Segment); ok {
				total += seg.Memsz
			}
		}
		return total, nil
	}
	return 0, os.ErrInvalid
}
