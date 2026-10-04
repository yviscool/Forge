#include <bits/stdc++.h>
using namespace std;
// 文件 IO 错解：读了不写 -> 无输出文件。
int main() {
    long long a, b;
    FILE *f = fopen("p.in", "r");
    if (!f) return 1;
    fscanf(f, "%lld%lld", &a, &b);
    fclose(f);
    return 0;
}
