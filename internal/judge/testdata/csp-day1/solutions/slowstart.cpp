#include <bits/stdc++.h>
using namespace std;
// 慢启动守卫：先空转约 0.3s 再正确作答，限时充足时必须 AC（防启动开销误杀）。
int main() {
    for (volatile long long i = 0; i < 300000000LL; i++) { }
    long long a, b;
    if (!(cin >> a >> b)) return 0;
    cout << a + b;
    return 0;
}
