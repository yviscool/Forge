#include <algorithm>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <string>
#include <vector>
#include <fstream>
using namespace std;
// 交互器(部分分协议测试):正常完成交互后向 stderr 写 testlib 打分标记
// partially correct (50) 并 exit 0 -> 部分分(满分一半)。
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
    if (got != a + b) return 1;
    std::cerr << "partially correct (50)" << std::endl;
    return 0;
}
