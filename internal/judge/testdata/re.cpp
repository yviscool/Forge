#include <bits/stdc++.h>
using namespace std;
int main() {
    int *p = nullptr;
    *p = 42; // null deref -> RE
    return 0;
}
