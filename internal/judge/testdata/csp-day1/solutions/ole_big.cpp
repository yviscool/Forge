#include <bits/stdc++.h>
using namespace std;
// 输出 20MB：验证 OLE 截断。
int main() {
    std::string chunk(1000, 'x');
    for (int i = 0; i < 20000; i++) std::cout << chunk;
    return 0;
}
