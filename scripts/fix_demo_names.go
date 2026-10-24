//go:build ignore

// 一次性数据修复：把早前经 GBK 终端种入的乱码比赛名改成正确 UTF-8。
// 用法：go run scripts/fix_demo_names.go <db路径>
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: go run scripts/fix_demo_names.go <db>")
		os.Exit(1)
	}
	db, err := sql.Open("sqlite", "file:"+os.Args[1]+"?_pragma=busy_timeout(5000)")
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, data FROM contests`)
	if err != nil {
		fmt.Println("query:", err)
		os.Exit(1)
	}
	type raw struct {
		ID   string
		Data string
	}
	var all []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.ID, &r.Data); err != nil {
			fmt.Println("scan:", err)
			os.Exit(1)
		}
		all = append(all, r)
	}
	rows.Close()

	fixed := 0
	for _, r := range all {
		var c map[string]any
		if err := json.Unmarshal([]byte(r.Data), &c); err != nil {
			continue
		}
		name, _ := c["name"].(string)
		// U+FFFD 出现即早前 GBK 种入的乱码名。
		if !strings.ContainsRune(name, 0xFFFD) {
			continue
		}
		// 用 ID 顺序重建可读名字。
		c["name"] = "CSP-J 模拟认证"
		if desc, _ := c["description"].(string); strings.ContainsRune(desc, 0xFFFD) {
			c["description"] = "桌面验收演示赛"
		}
		out, err := json.Marshal(c)
		if err != nil {
			continue
		}
		if _, err := db.Exec(`UPDATE contests SET data=? WHERE id=?`, string(out), r.ID); err != nil {
			fmt.Println("update", r.ID, err)
			continue
		}
		fixed++
	}
	fmt.Printf("fixed %d contests\n", fixed)

	// 提交里的 userName 快照同样可能带 U+FFFD（旧提交）。
	res, err := db.Exec(`UPDATE submissions SET data =
		CASE WHEN data LIKE '%' || char(65533) || '%'
		THEN json_set(data, '$.userName', '演示考生')
		ELSE data END
		WHERE data LIKE '%' || char(65533) || '%'`)
	if err == nil {
		n, _ := res.RowsAffected()
		fmt.Printf("fixed %d submissions\n", n)
	} else {
		fmt.Println("submissions fix skipped:", err)
	}
}
