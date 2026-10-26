// 静态内存预检路径(全平台确定性):BSS 大数组,起跑前即 MLE,
// 不依赖运行时墙的行为差异。对标 LemonLime watcher 静态内存分析。
// 注意必须用运行期偏移访问全数组,否则 -O2 会把只访问首字节的
// 静态数组整体收缩消除(bss 只剩几字节)。
#include <cstdio>
static char g[300 << 20];
int main(int argc, char **) {
    volatile char *p = g + (argc << 20);
    *p = 1;
    std::printf("%d\n", *p);
    return 0;
}
