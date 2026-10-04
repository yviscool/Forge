#include <bits/stdc++.h>
using namespace std;
// 典型慢：选择排序 O(n^2) 无早退 + 正确贪心，有序数据照样超时。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    vector<pair<long long,long long>> a(n);
    for (int i = 0; i < n; i++) cin >> a[i].first >> a[i].second;
    for (int i = 0; i < n; i++) {
        int m = i;
        for (int j = i + 1; j < n; j++) {
            if (a[j].second < a[m].second ||
                (a[j].second == a[m].second && a[j].first < a[m].first)) m = j;
        }
        if (m != i) swap(a[i], a[m]);
    }
    long long ans = 0, last = LLONG_MIN / 4;
    for (auto &p : a) {
        if (p.first > last) { ans++; last = p.second; }
    }
    cout << ans;
    return 0;
}
