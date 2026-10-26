// darwin 轻量语料:纯 <iostream>,clang 快编,验证全栈判题(AC)。
#include <iostream>
int main() {
    long long a, b;
    if (!(std::cin >> a >> b)) return 1;
    std::cout << a + b << std::endl;
    return 0;
}
