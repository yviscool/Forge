package main

// 会话探针：清空/列出 sessions，用来判断桌面主窗的 ?dt= 兑换（POST）
// 是否真的打到了服务端——不需要看 webview 里的画面。
//
//	go run ./scripts/devtools/qsess -clear
//	go run ./scripts/devtools/qsess
import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	clear := flag.Bool("clear", false, "delete all sessions first")
	flag.Parse()

	db, err := sql.Open("sqlite", "file:data/forge.db?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if *clear {
		if _, err := db.Exec(`DELETE FROM sessions`); err != nil {
			log.Fatal(err)
		}
		fmt.Println("sessions cleared")
		return
	}
	rows, err := db.Query(`SELECT user_id, json_extract(data,'$.createdAt') FROM sessions ORDER BY rowid DESC LIMIT 8`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uid, created string
		if err := rows.Scan(&uid, &created); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s  %s\n", created, uid)
		n++
	}
	if n == 0 {
		fmt.Println("(no sessions)")
	}
}
