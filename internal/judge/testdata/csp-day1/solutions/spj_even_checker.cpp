#include "testlib_min.h"
#include <string>
// 多解题特判：输入 n，输出 [0, n] 内任意偶数。
int main(int argc, char **argv) {
    forge_checker::check_argc(argc);
    long long n = std::stoll(forge_checker::read_all(argv[1]));
    std::string out = forge_checker::read_all(argv[2]);
    long long o = 0;
    try { o = std::stoll(out); }
    catch (...) { forge_checker::wa("not a number"); }
    if (o < 0 || o > n) forge_checker::wa("out of range");
    if (o % 2 != 0) forge_checker::wa("not even");
    forge_checker::ok("even ok");
    return 0;
}
