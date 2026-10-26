#include <iostream>
#include <cstdio>
#include <cstring>
// 交互正解变体:正确作答后 spawn 一个握住 stderr 睡 30s 的子进程,父立即
// 退出——复现"孙进程握 stderr 管道"场景,验证交互路径的 WaitDelay 防死锁。
// Windows 下 Job ActiveProcessLimit=1 会拦住 spawn(忽略失败,行为退化为
// 普通正解)。
#ifdef _WIN32
#include <windows.h>
#include <string>
static void spawn_holder() {
    char path[MAX_PATH];
    GetModuleFileNameA(NULL, path, MAX_PATH);
    std::string cmd = std::string("\"") + path + "\" child";
    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    ZeroMemory(&si, sizeof(si));
    si.cb = sizeof(si);
    ZeroMemory(&pi, sizeof(pi));
    char buf[4096];
    strncpy(buf, cmd.c_str(), sizeof(buf) - 1);
    CreateProcessA(NULL, buf, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi);
    if (pi.hThread) CloseHandle(pi.hThread);
    if (pi.hProcess) CloseHandle(pi.hProcess);
}
int main(int argc, char **argv) {
    if (argc > 1 && std::strcmp(argv[1], "child") == 0) {
        Sleep(30000);
        return 0;
    }
    long long a, b;
    if (!(std::cin >> a >> b)) return 1;
    std::cout << a + b << std::endl;
    spawn_holder();
    return 0;
}
#else
#include <cstring>
#include <unistd.h>
static void spawn_holder() {
    if (fork() == 0) {
        // 子进程握住(继承的)stderr 睡 30s,刻意不关。
        sleep(30);
        _exit(0);
    }
}
int main(int argc, char **argv) {
    if (argc > 1 && std::strcmp(argv[1], "child") == 0) {
        sleep(30);
        return 0;
    }
    long long a, b;
    if (!(std::cin >> a >> b)) return 1;
    std::cout << a + b << std::endl;
    spawn_holder();
    return 0;
}
#endif
