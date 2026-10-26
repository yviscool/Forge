// fork 炸弹 containment 探针：连发 500 个睡眠子进程，spawn 被墙即
// 收割已产出子进程并 exit(2) -> 三平台一致的快速 RE。
//   Windows Job ActiveProcessLimit=1:首个 spawn 即被拦;
//   Linux prlimit RLIMIT_NPROC=64 / bwrap user-ns:同 UID(或命名空间)
//   进程配额远小于 500,必然 spawn 失败。
// 子进程先关掉 stdout/stderr 再睡:否则孤儿进程握住管道写端,Go 侧
// cmd.Wait 要等 EOF,RE 判定会被拖满 30s。
#ifdef _WIN32
#include <windows.h>
#include <string>
#include <vector>
static HANDLE child_handle = NULL;
int spawn_child() {
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
        return -1;
    }
    CloseHandle(pi.hThread);
    child_handle = pi.hProcess;
    return 0;
}
int main() {
    for (int i = 0; i < 500; i++) {
        if (spawn_child() != 0) {
            std::printf("blocked at %d\n", i);
            if (child_handle) TerminateProcess(child_handle, 1);
            return 2;
        }
    }
    std::printf("unlimited\n");
    if (child_handle) TerminateProcess(child_handle, 1);
    return 3;
}
#else
#include <sys/wait.h>
#include <unistd.h>
int spawn_child() {
    pid_t pid = fork();
    if (pid < 0) return -1;
    if (pid == 0) {
        // 子进程释放 stdout/stderr:否则孤儿进程握住管道写端,Go 侧
        // cmd.Wait 要等管道 EOF,RE 判定会被拖满 30s。
        close(1);
        close(2);
        sleep(30);
        _exit(0);
    }
    return pid;
}
int main() {
    std::vector<pid_t> kids;
    for (int i = 0; i < 500; i++) {
        pid_t pid = spawn_child();
        if (pid < 0) {
            std::printf("blocked at %d\n", i);
            for (pid_t k : kids) kill(k, SIGKILL);
            return 2;
        }
        kids.push_back(pid);
    }
    std::printf("unlimited\n");
    for (pid_t k : kids) kill(k, SIGKILL);
    return 3;
}
#endif
