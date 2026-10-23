package desktop

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/skip2/go-qrcode"
)

//go:embed assets
var assetsFS embed.FS

// TerminalQR 生成可在终端/日志直接展示的 ASCII 二维码（教师机控制台扫码即达）。
func TerminalQR(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("qr content is empty")
	}
	code, err := qrcode.New(content, qrcode.Low)
	if err != nil {
		return "", err
	}
	ascii := code.ToString(false)
	return ascii, nil
}

// trayIcon 生成托盘图标：内嵌 PNG 资产，Windows 侧包一层 ICO 头。
func trayIcon() ([]byte, error) {
	raw, err := assetsFS.ReadFile("assets/tray-32.png")
	if err != nil {
		return nil, err
	}
	return icoFromPNG(raw)
}

// icoFromPNG 把单张 PNG 包装成 ICO（Vista+ 支持 PNG 直嵌，无需重采样）。
func icoFromPNG(pngBytes []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 255 || h > 255 {
		return nil, fmt.Errorf("icon too large: %dx%d", w, h)
	}
	out := &bytes.Buffer{}
	// ICONDIR：保留字段 0 + 类型 1（图标）+ 数量 1。
	out.Write([]byte{0, 0, 1, 0, 1, 0})
	// ICONDIRENTRY：宽/高（0 表示 256）、色板 0、保留 0、色板数 1、位深 32、
	// 数据长度与偏移。
	entry := []byte{byte(w % 256), byte(h % 256), 0, 0, 1, 0, 32, 0}
	size := uint32(len(pngBytes))
	entry = append(entry, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	offset := uint32(22)
	entry = append(entry, byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24))
	out.Write(entry)
	out.Write(pngBytes)
	return out.Bytes(), nil
}

// GenerateIconPNG 供测试与资源再生成使用：画一个 Forge 蓝 32x32 圆角方块。
func GenerateIconPNG() ([]byte, error) {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	forge := color.RGBA{0x14, 0x28, 0x3d, 0xff}
	accent := color.RGBA{0x40, 0x5d, 0xff, 0xff}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// 圆角：四角 6px 半径内留空。
			corner := false
			for _, cx := range []int{5, size - 6} {
				for _, cy := range []int{5, size - 6} {
					dx, dy := x-cx, y-cy
					if dx*dx+dy*dy > 36 && ((x < 6 || x > size-7) && (y < 6 || y > size-7)) {
						corner = true
					}
				}
			}
			if corner {
				continue
			}
			if x >= 8 && x <= 23 && y >= 14 && y <= 17 {
				img.Set(x, y, accent) // 横杠：Forge 标识
			} else {
				img.Set(x, y, forge)
			}
		}
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
