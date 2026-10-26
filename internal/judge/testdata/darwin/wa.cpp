// darwin 轻量语料:错误答案(WA)。
#include <iostream>
int main() {
    long long a, b;
    if (!(std::cin >> a >> b)) return 1;
    std::cout << a + b + 1 << std::endl;
    return 0;
}
