#include <bits/stdc++.h>
using namespace std;
// 并行偷钟探针：4 线程各忙等，wall 短但 CPU 爆表，必须按 CPU 判 TLE。
int main() {
    auto burn = []() {
        auto t0 = std::chrono::steady_clock::now();
        volatile unsigned long long x = 0;
        while (std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - t0).count() < 600) {
            x++;
        }
    };
    std::thread t1(burn), t2(burn), t3(burn), t4(burn);
    t1.join(); t2.join(); t3.join(); t4.join();
    cout << "done";
    return 0;
}

