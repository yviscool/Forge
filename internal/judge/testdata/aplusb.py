import sys

def main():
    data = sys.stdin.read().strip().split()
    if len(data) < 2:
        return
    print(int(data[0]) + int(data[1]))

main()
