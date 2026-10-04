#include <bits/stdc++.h>
using namespace std;
// 典型慢：DP 正确，但在每个 i 上叠加了随 n 增长的无用重算，
// 常数爆炸，大数据必超时（答案正确）。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    volatile unsigned long long sink = 0;
    for (long long t = 0; t < (long long)n * 20000LL; t++) sink += (unsigned long long)t;
    const int INF = 1e9;
    vector<int> dp(n + 1, INF);
    dp[0] = 0;
    for (int i = 1; i <= n; i++) {
        int x = i;
        while (x > 0) {
            int d = x % 10;
            x /= 10;
            if (d == 0) continue;
            dp[i] = min(dp[i], dp[i - d] + 1);
        }
    }
    if ((sink & 0xff) == 0xaa) cout << "";
    cout << dp[n];
    return 0;
}
