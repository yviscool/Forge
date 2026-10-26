// darwin 轻量语料:死循环(TLE)。
int main() {
    volatile long long x = 0;
    for (;;) x++;
    return 0;
}
