#include <bits/stdc++.h>
using namespace std;
// 交互器：把输入文件内容发给选手，读回一行校验 a+b。
int main(int argc, char **argv) {
    if (argc != 2) return 3;
    long long a, b;
    {
        std::ifstream f(argv[1]);
        if (!(f >> a >> b)) return 3;
    }
    std::cout << a << ' ' << b << std::endl;
    long long got;
    if (!(std::cin >> got)) return 1;
    if (got == a + b) return 0;
    return 1;
}
