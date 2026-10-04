import sys


def main():
    data = sys.stdin.read().strip()
    if not data:
        return
    n = int(data.split()[0])
    # 典型错：循环内忘记更新 n，死循环 -> TLE（初学者最高频超时）。
    ans = 0
    while n > 0:
        x = n
        best = 0
        while x > 0:
            d = x % 10
            x //= 10
            if d > best:
                best = d
        ans += 1


main()
