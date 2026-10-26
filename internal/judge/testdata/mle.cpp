#include <bits/stdc++.h>
using namespace std;
int main() {
    // commit 1MB at a time: Windows 命中 Job 内存墙截杀；Linux 触发
    // std::bad_alloc -> terminate -> SIGABRT（stderr 带 bad_alloc），
    // 两条路径都稳定落 MLE，而非 nothrow 返回 99 的近墙 RE。
    vector<char *> blocks;
    for (;;) {
        blocks.push_back(new char[1 << 20]);
        memset(blocks.back(), 1, 1 << 20);
    }
}
