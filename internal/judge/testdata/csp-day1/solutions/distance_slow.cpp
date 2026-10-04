#include <bits/stdc++.h>
using namespace std;
// 典型慢：从0向上线性试探代替二分，答案稍大即超时。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n, k;
    if (!(cin >> n >> k)) return 0;
    vector<long long> x(n);
    for (int i = 0; i < n; i++) cin >> x[i];
    sort(x.begin(), x.end());
    auto ok = [&](long long d) {
        int cnt = 1;
        long long last = x[0];
        for (int i = 1; i < n; i++) {
            if (x[i] - last >= d) { cnt++; last = x[i]; }
        }
        return cnt >= k;
    };
    long long d = 0;
    while (ok(d + 1)) d++;
    cout << d;
    return 0;
}
