// fork 炸弹 containment 探针：spawn 睡眠子进程，spawn 失败即 exit(2)。
// Windows Job ActiveProcessLimit=1 下第二个进程即被拦 -> 快速 RE。
#ifdef _WIN32
#include <windows.h>
#include <string>
int spawn_self(const char *self) {
    std::string cmd = std::string("\"") + self + "\" child";
    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    ZeroMemory(&si, sizeof(si));
    si.cb = sizeof(si);
    ZeroMemory(&pi, sizeof(pi));
    char buf[4096];
    strncpy(buf, cmd.c_str(), sizeof(buf) - 1);
    if (!CreateProcessA(NULL, buf, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) {
        return -1;
    }
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    return 0;
}
#else
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
int spawn_self(const char *self) {
    pid_t pid = fork();
    if (pid < 0) return -1;
    if (pid == 0) {
        execl(self, self, "child", (char *)0);
        _exit(127);
    }
    return 0;
}
#endif
#include <cstdio>
#include <cstring>
#include <string>
#include <thread>
#include <chrono>
int main(int argc, char **argv) {
    if (argc > 1 && std::strcmp(argv[1], "child") == 0) {
        std::this_thread::sleep_for(std::chrono::seconds(30));
        return 0;
    }
    for (int i = 0; i < 3; i++) {
        if (spawn_self(argv[0]) != 0) {
            std::printf("blocked at %d\n", i);
            return 2; //  spawn 被墙 -> 快速 RE（预期行为）
        }
    }
    std::this_thread::sleep_for(std::chrono::seconds(30));
    return 0;
}
