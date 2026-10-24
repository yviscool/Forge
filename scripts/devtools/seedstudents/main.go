package main

// 本机演示专用：批量创建真实中文姓名的学生账号，分布在不同省份。
// 走 store 层写入，保证 users.data 的 JSON 形态与运行期完全一致。
//
//	go run ./scripts/devtools/seedstudents -db data/forge.db -password student2026
//
// 应用必须处于停止状态（SQLite 单写者）。
import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yviscool/forge/internal/adapters/store/sqlite"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
)

type seed struct {
	Name     string
	Username string
	Province string
}

// seeds 每省两名，覆盖 10 个考区；姓名取常见真实姓名，登录名用全拼。
var seeds = []seed{
	{"张伟", "zhangwei", "HA"},
	{"李思远", "lisiyuan", "HA"},
	{"王梓涵", "wangzihan", "SD"},
	{"刘雨桐", "liuyutong", "SD"},
	{"陈嘉懿", "chenjiayi", "JS"},
	{"杨浩然", "yanghaoran", "JS"},
	{"赵一诺", "zhaoyinuo", "ZJ"},
	{"黄子轩", "huangzixuan", "ZJ"},
	{"周欣怡", "zhouxinyi", "SC"},
	{"吴天佑", "wutianyou", "SC"},
	{"徐若曦", "xuruoxi", "GD"},
	{"孙铭泽", "sunmingze", "GD"},
	{"马嘉祺", "majiaqi", "BJ"},
	{"朱雨欣", "zhuyuxin", "BJ"},
	{"胡俊杰", "hujunjie", "HB"},
	{"郭可馨", "guokexin", "HB"},
	{"何沐阳", "hemuyang", "SN"},
	{"高瑞泽", "gaoruize", "SN"},
	{"林清月", "linqingyue", "FJ"},
	{"罗文博", "luowenbo", "HN"},
}

func main() {
	dbPath := flag.String("db", "data/forge.db", "sqlite 数据库路径")
	password := flag.String("password", "student2026", "统一初始口令")
	flag.Parse()

	if err := auth.ValidatePassword(*password); err != nil {
		log.Fatalf("password: %v", err)
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
	taken := make(map[string]bool, len(users))
	for _, u := range users {
		taken[strings.ToLower(u.Username)] = true
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), auth.Cost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	created := 0
	for _, s := range seeds {
		if _, ok := domain.ProvinceByCode(s.Province); !ok {
			log.Fatalf("unknown province %q for %s", s.Province, s.Name)
		}
		username := s.Username
		for i := 2; taken[strings.ToLower(username)]; i++ {
			username = fmt.Sprintf("%s_%d", s.Username, i)
		}
		u, err := store.CreateUser(domain.User{
			ID:        store.NextID("usr"),
			Username:  username,
			Name:      s.Name,
			Role:      domain.RoleStudent,
			Province:  s.Province,
			Groups:    []string{},
			CreatedAt: time.Now(),
		})
		if err != nil {
			log.Fatalf("create %s: %v", s.Name, err)
		}
		if err := store.SetPasswordHash(u.ID, string(hash)); err != nil {
			log.Fatalf("password for %s: %v", u.Username, err)
		}
		if err := store.SetPasswordPlain(u.ID, *password); err != nil {
			log.Fatalf("plain password for %s: %v", u.Username, err)
		}
		taken[strings.ToLower(username)] = true
		created++
		fmt.Printf("%-4s %-12s %-4s %s\n", s.Province, u.Username, u.ID, u.Name)
	}
	fmt.Printf("\n已创建 %d 名学生，统一初始口令 %q\n", created, *password)
}
