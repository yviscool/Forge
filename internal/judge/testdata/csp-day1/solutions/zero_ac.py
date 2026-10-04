import sys


def main():
    data = sys.stdin.read().strip()
    if not data:
        return
    n = int(data.split()[0])
    INF = 10 ** 9
    dp = [INF] * (n + 1)
    dp[0] = 0
    for i in range(1, n + 1):
        x = i
        while x > 0:
            d = x % 10
            x //= 10
            if d == 0:
                continue
            if dp[i - d] + 1 < dp[i]:
                dp[i] = dp[i - d] + 1
    print(dp[n])


main()
