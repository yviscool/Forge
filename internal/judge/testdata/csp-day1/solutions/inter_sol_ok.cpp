#include <algorithm>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <string>
#include <vector>
using namespace std;
// 交互题正解：读交互器发来的 a b，回和。
int main() {
    long long a, b;
    if (!(cin >> a >> b)) return 1;
    cout << a + b << endl;
    return 0;
}
