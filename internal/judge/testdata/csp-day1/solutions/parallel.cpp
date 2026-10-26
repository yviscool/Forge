#include <algorithm>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <string>
#include <vector>
#include <thread>
using namespace std;
// 并行偷钟探针：4 线程各烧固定迭代量（约 0.4-1.2s CPU/线程，视主频），
// 而非固定 wall 忙等——调度受限的宿主上烧不完就撞墙钟 TLE，
// 烧完则总 CPU 必然超过限额，两种路径都不会误判 AC。
int main() {
    auto burn = []() {
        volatile unsigned long long x = 0;
        for (unsigned long long i = 0; i < 1200000000ULL; ++i) {
            x += i;
        }
    };
    std::thread t1(burn), t2(burn), t3(burn), t4(burn);
    t1.join(); t2.join(); t3.join(); t4.join();
    cout << "done";
    return 0;
}
