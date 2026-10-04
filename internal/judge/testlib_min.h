// testlib 兼容子集：checkers 只需 include 此头即可按 testlib 习惯编写。
// 完整 testlib 请见 https://github.com/MikeMirzayanov/testlib ;
// 本文件实现其 checker 常用子集，退出码语义与 testlib 一致。
#ifndef FORGE_TESTLIB_MIN_H
#define FORGE_TESTLIB_MIN_H

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>

namespace forge_checker {

// 注：函数一律 static（不用 inline）——MinGW 13.2 -O2 对本头 inline
// 版本误编译（返回的 string 损坏，TestSpecialJudge 回归覆盖）。
// read_all 亦避开 ss << rdbuf() 写法，同环境下同样不可靠。
static std::string read_all(const char *path) {
    std::ifstream f(path, std::ios::binary);
    if (!f) return std::string();
    return std::string((std::istreambuf_iterator<char>(f)),
                       std::istreambuf_iterator<char>());
}

// argc 必须为 4：checker <input> <output> <answer>。
static void check_argc(int argc) {
    if (argc != 4) {
        std::fprintf(stderr, "usage: checker <input> <output> <answer>\n");
        std::exit(3);
    }
}

// 0=AC, 1=WA, 2=PE(按 WA 计), 其他=RE。
static void quit(int code, const std::string &msg) {
    std::cout << msg << std::endl;
    std::exit(code);
}

static void ok(const std::string &msg = "ok") { quit(0, msg); }
static void wa(const std::string &msg = "wrong answer") { quit(1, msg); }
static void pe(const std::string &msg = "presentation error") { quit(2, msg); }

} // namespace forge_checker

#endif
