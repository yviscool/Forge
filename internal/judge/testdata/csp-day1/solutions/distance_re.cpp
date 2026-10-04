#include <bits/stdc++.h>
using namespace std;
int main() {
    int n, k;
    if (!(cin >> n >> k)) return 0;
    int *p = nullptr;
    *p = n + k; // 确定性崩溃 -> RE
    return 0;
}
