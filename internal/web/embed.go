// Package web 只做一件事：把前端构建产物编译进单二进制。
package web

import "embed"

//go:embed dist
var Dist embed.FS
