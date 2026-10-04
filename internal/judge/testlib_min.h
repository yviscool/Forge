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

inline std::string read_all(const char *path) {
    std::ifstream f(path, std::ios::binary);
    std::ostringstream ss;
    ss << f.rdbuf();
    return ss.str();
}

// argc 必须为 4：checker <input> <output> <answer>。
inline void check_argc(int argc) {
    if (argc != 4) {
        std::fprintf(stderr, "usage: checker <input> <output> <answer>\n");
        std::exit(3);
    }
}

// 0=AC, 1=WA, 2=PE(按 WA 计), 其他=RE。
inline void quit(int code, const std::string &msg) {
    std::cout << msg << std::endl;
    std::exit(code);
}

inline void ok(const std::string &msg = "ok") { quit(0, msg); }
inline void wa(const std::string &msg = "wrong answer") { quit(1, msg); }
inline void pe(const std::string &msg = "presentation error") { quit(2, msg); }

} // namespace forge_checker

#endif
