#include <bits/stdc++.h>
using namespace std;
// 分点限额探针：n==2 死循环，其余回声。
int main() {
    long long n;
    if (!(cin >> n)) return 0;
    if (n == 2) { volatile long long x = 0; while (true) x++; }
    cout << n;
    return 0;
}
