#include <bits/stdc++.h>
using namespace std;
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if (!(cin >> n)) return 0;
    vector<pair<long long,long long>> a(n);
    for (int i = 0; i < n; i++) cin >> a[i].first >> a[i].second;
    sort(a.begin(), a.end(), [](auto &x, auto &y){
        if (x.second != y.second) return x.second < y.second;
        return x.first < y.first;
    });
    long long ans = 0, last = LLONG_MIN / 4;
    for (auto &p : a) {
        if (p.first > last) { ans++; last = p.second; }
    }
    cout << ans;
    return 0;
}
