#include <bits/stdc++.h>
using namespace std;
// 多解题正解：输出 [0,n] 内偶数。
int main() {
    long long n;
    if (!(cin >> n)) return 0;
    cout << (n % 2 == 0 ? n : n - 1);
    return 0;
}
