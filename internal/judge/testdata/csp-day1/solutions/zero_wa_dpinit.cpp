#include <bits/stdc++.h>
using namespace std;
// 典型错：dp[0]=1 起手，答案整体偏大1。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    const int INF = 1e9;
    vector<int> dp(n + 1, INF);
    dp[0] = 1;
    for (int i = 1; i <= n; i++) {
        int x = i;
        while (x > 0) {
            int d = x % 10;
            x /= 10;
            if (d == 0) continue;
            dp[i] = min(dp[i], dp[i - d] + 1);
        }
    }
    cout << dp[n];
    return 0;
}
