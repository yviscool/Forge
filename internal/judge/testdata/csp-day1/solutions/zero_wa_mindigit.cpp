#include <bits/stdc++.h>
using namespace std;
// 典型错：每次减最小非零数位（能结束，但步数偏多）。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    int ans = 0;
    while (n > 0) {
        int x = n, mn = 10;
        while (x > 0) { int d = x % 10; x /= 10; if (d != 0) mn = min(mn, d); }
        n -= mn;
        ans++;
    }
    cout << ans;
    return 0;
}
