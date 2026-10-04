#include <bits/stdc++.h>
using namespace std;
// Lemon helloworld 式 verdict 分发器：一次编译，输入决定行为。
// 输入 1 -> AC("AC")；2 -> WA("WA")；3 -> MLE(内存墙截杀)；
// 4 -> TLE(死循环)；5 -> RE(空指针)；6 -> OLE(20MB 输出)。
// 期望输出统一为 "AC"。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int s;
    if (!(cin >> s)) return 0;
    switch (s) {
        case 1:
            cout << "AC";
            break;
        case 2:
            cout << "WA";
            break;
        case 3: {
            // 1MB 渐进提交，翻过内存墙时被 Job 截杀 -> MLE（整块大申请会被
            // 直接拒绝而非截杀，测不出墙）。
            std::vector<char *> blocks;
            for (int i = 0; i < 400; i++) {
                char *p = new char[1 << 20];
                std::memset(p, 1, 1 << 20);
                blocks.push_back(p);
            }
            std::cout << "AC";
            break;
        }
        case 4: {
            volatile long long x = 0;
            while (true) x++;
            break;
        }
        case 5: {
            int *p = nullptr;
            *p = 42;
            break;
        }
        case 6: {
            std::string chunk(1000, 'x');
            for (int i = 0; i < 20000; i++) std::cout << chunk;
            break;
        }
        default:
            break;
    }
    return 0;
}
