package judge

import "runtime"

// DefaultWorkers 判题 worker 缺省并发数：物理核数。SMT 超订阅会让两个
// 超线程争抢同一物理核、污染 CPU-time 计时（对标 LemonLime cputopology
// 按 CPU 拓扑取缺省）。探测失败回退逻辑核数，最小 1。
// FORGE_JUDGE_WORKERS 显式配置优先。
func DefaultWorkers() int {
	n := physicalCores()
	if n < 1 {
		n = runtime.NumCPU()
	}
	if n < 1 {
		n = 1
	}
	return n
}
