// 父退子留探针:立即产出一个握住 stdout 睡 30s 的子进程,然后自己正常退出。
// 无 WaitDelay 时 cmd.Wait 会阻塞在管道排空上直到子进程 30s 睡完——
// 回归测试断言判题在 20s 内完成且 verdict 正常。
#ifdef _WIN32
#include <windows.h>
#include <cstdio>
#include <cstring>
#include <string>
int main(int argc, char **argv) {
    if (argc > 1 && std::strcmp(argv[1], "child") == 0) {
        Sleep(30000);
        return 0;
    }
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
    if (!CreateProcessA(NULL, buf, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) {
        return 2;
    }
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    return 0; // 父进程立即退出,子进程留场握管道
}
#else
#include <cstdio>
#include <cstring>
#include <unistd.h>
int main(int argc, char **argv) {
    if (argc > 1 && std::strcmp(argv[1], "child") == 0) {
        sleep(30);
        return 0;
    }
    if (fork() == 0) {
        // 子进程刻意不关 stdout:握住管道,复现孤儿管道场景。
        sleep(30);
        _exit(0);
    }
    return 0;
}
#endif
