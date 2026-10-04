#include <bits/stdc++.h>
using namespace std;
// 典型错：按左端点贪心（取最早开始区间的右端点），嵌套区间翻车。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    vector<pair<long long,long long>> a(n);
    for (int i = 0; i < n; i++) cin >> a[i].first >> a[i].second;
    sort(a.begin(), a.end());
    long long ans = 0, last = LLONG_MIN / 4;
    for (auto &p : a) {
        if (p.first > last) { ans++; last = p.second; }
    }
    cout << ans;
    return 0;
}
