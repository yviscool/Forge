#include <bits/stdc++.h>
using namespace std;
int main() {
    // commit 1MB at a time so the 64MB job wall kills us mid-run -> MLE
    vector<char *> blocks;
    for (int i = 0; i < 512; i++) {
        char *p = new (nothrow) char[1 << 20];
        if (!p) return 99; // allocation refused (also OOM, but wall-kill is the expected path)
        memset(p, 1, 1 << 20);
        blocks.push_back(p);
    }
    cout << "alive";
    return 0;
}
