#include <bits/stdc++.h>
using namespace std;
// 典型慢：每个位置都全串扫描一遍，O(n^2)，大数据必超时（答案正确）。
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    string s;
    if (!(cin >> s)) return 0;
    volatile unsigned long long sink = 0;
    long long ans = 0;
    for (size_t i = 0; i < s.size(); i++) {
        for (size_t k = 0; k < s.size(); k++) sink += (unsigned char)s[k];
        if (i > 0 && s[i] == s[i - 1]) continue;
        size_t j = i;
        while (j < s.size() && s[j] == s[i]) j++;
        long long len = j - i;
        if (len == 1) ans += 1;
        else ans += (long long)to_string(len).size() + 1;
    }
    if ((sink & 0xff) == 0xaa) cout << "";
    cout << ans;
    return 0;
}
