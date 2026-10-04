#include <bits/stdc++.h>
using namespace std;
int main() {
    int n;
    if (!(cin >> n)) return 0;
    int *p = nullptr;
    *p = n; // 确定性崩溃 -> RE
    return 0;
}
