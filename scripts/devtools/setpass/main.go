package main

// 本机演示专用：绕过 ValidatePassword（产品策略要求 ≥10 字符），
// 直接把某个账号的口令改成指定值。仅供教师机本地演示库使用。
//
//	go run ./scripts/devtools/setpass -user admin -password admin
//
// 应用必须处于停止状态（SQLite 单写者）。
import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/yviscool/forge/internal/adapters/store/sqlite"
	"github.com/yviscool/forge/internal/auth"
)

func main() {
	dbPath := flag.String("db", "data/forge.db", "sqlite 数据库路径")
	user := flag.String("user", "admin", "登录名（username）")
	password := flag.String("password", "", "新口令（故意跳过强度校验）")
	flag.Parse()

	if *password == "" {
		log.Fatal("-password required")
	}
	store, err := sqlite.Open(*dbPath)
	if err != nil {
		log.Fatalf("open %s: %v", *dbPath, err)
	}
	defer func() { _ = store.Close() }()

	users, err := store.ListUsers()
	if err != nil {
		log.Fatalf("list users: %v", err)
	}
	target := ""
	for _, u := range users {
		if u.Username == *user {
			target = u.ID
			break
		}
	}
	if target == "" {
		fmt.Fprintf(os.Stderr, "user %q not found in %s\n", *user, *dbPath)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), auth.Cost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	if err := store.SetPasswordHash(target, string(hash)); err != nil {
		log.Fatalf("set hash: %v", err)
	}
	if err := store.DeleteSessionsForUser(target); err != nil {
		log.Fatalf("revoke sessions: %v", err)
	}
	fmt.Printf("ok: %s (%s) 口令已设为 %q（%d 字符，低于产品策略下限 %d）\n",
		*user, target, *password, len([]rune(*password)), auth.MinPasswordLength)
}
