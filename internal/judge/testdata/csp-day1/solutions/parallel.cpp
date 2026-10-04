#include <bits/stdc++.h>
using namespace std;
// 并行偷钟探针：4 线程各忙等，wall 短但 CPU 爆表，必须按 CPU 判 TLE。
int main() {
    auto burn = []() {
        volatile unsigned long long x = 0;
        for (long long i = 0; i < 600000000LL; i++) x += (unsigned long long)i;
    };
    std::thread t1(burn), t2(burn), t3(burn), t4(burn);
    t1.join(); t2.join(); t3.join(); t4.join();
    cout << "done";
    return 0;
}
