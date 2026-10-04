#include <bits/stdc++.h>
using namespace std;
// 文件 IO 正解：读 p.in 写 p.out。
int main() {
    long long a, b;
    FILE *f = fopen("p.in", "r");
    if (!f) return 1;
    fscanf(f, "%lld%lld", &a, &b);
    fclose(f);
    f = fopen("p.out", "w");
    if (!f) return 1;
    fprintf(f, "%lld", a + b);
    fclose(f);
    return 0;
}
