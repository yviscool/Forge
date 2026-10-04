#include <bits/stdc++.h>
using namespace std;
// 典型错：忘记排序，默认输入有序。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n, k;
    if (!(cin >> n >> k)) return 0;
    vector<long long> x(n);
    for (int i = 0; i < n; i++) cin >> x[i];
    auto ok = [&](long long d) {
        int cnt = 1;
        long long last = x[0];
        for (int i = 1; i < n; i++) {
            if (x[i] - last >= d) { cnt++; last = x[i]; }
        }
        return cnt >= k;
    };
    long long lo = 0, hi = 4000000000LL;
    while (lo + 1 < hi) {
        long long mid = lo + (hi - lo) / 2;
        if (ok(mid)) lo = mid;
        else hi = mid;
    }
    cout << lo;
    return 0;
}
