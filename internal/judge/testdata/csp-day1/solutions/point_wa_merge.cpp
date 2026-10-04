#include <bits/stdc++.h>
using namespace std;
// 典型错：把“合并有交区间计数”当答案（连通块数≠最少点数）。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    vector<pair<long long,long long>> a(n);
    for (int i = 0; i < n; i++) cin >> a[i].first >> a[i].second;
    sort(a.begin(), a.end());
    long long ans = 0, curR = LLONG_MIN / 4;
    for (auto &p : a) {
        if (ans == 0 || p.first > curR) { ans++; curR = p.second; }
        else curR = max(curR, p.second);
    }
    cout << ans;
    return 0;
}
