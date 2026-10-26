#include <iostream>
#include <unistd.h>
// 组杀 fixture：fork 一个长眠子进程后自身死循环。
// 硬墙超时后父进程与子进程都必须消亡（进程组杀，防孙进程逃逸）。
int main() {
    pid_t pid = fork();
    if (pid == 0) {
        for (;;) pause();
    }
    std::cout << pid << std::flush;
    volatile long long x = 0;
    for (;;) x++;
}
