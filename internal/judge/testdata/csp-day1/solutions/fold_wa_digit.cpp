#include <bits/stdc++.h>
using namespace std;
// 典型错：以为重复次数恒为一位数，多位数少算长度。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    string s;
    if (!(cin >> s)) return 0;
    long long ans = 0;
    for (size_t i = 0; i < s.size();) {
        size_t j = i;
        while (j < s.size() && s[j] == s[i]) j++;
        long long len = j - i;
        if (len == 1) ans += 1;
        else ans += 2;
        i = j;
    }
    cout << ans;
    return 0;
}
