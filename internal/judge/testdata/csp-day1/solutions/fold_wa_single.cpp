#include <bits/stdc++.h>
using namespace std;
// 典型错：长度为1的段也写成“1a”，多算一位。
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
        ans += (long long)to_string(len).size() + 1;
        i = j;
    }
    cout << ans;
    return 0;
}
