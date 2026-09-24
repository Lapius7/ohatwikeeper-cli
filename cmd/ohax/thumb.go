package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

// 画像を「▀」(上半分=文字色、下半分=背景色)で1文字に2ピクセルずつ描いて端末に表示する。

var imgClient = &http.Client{Timeout: 10 * time.Second}

func fetchImage(url string) (image.Image, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ohax-cli/"+version)
	resp, err := imgClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, io.ErrUnexpectedEOF
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, 8<<20))
	return img, err
}

// twimgSmall はpbs.twimg.comの画像URLを小さいJPEG版にする(webpはGo標準で読めないため)
func twimgSmall(u, size string) string {
	if !strings.Contains(u, "pbs.twimg.com/media/") {
		return u
	}
	base, _, _ := strings.Cut(u, "?")
	return base + "?name=" + size
}

// halfBlock は画像を中央で正方形に切り抜き、cols×rows文字(cols×rows*2ピクセル)で描く。
// round=trueなら円形に切り抜く(アバター用)
func halfBlock(img image.Image, cols, rows int, round bool) []string {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	ph := rows * 2
	px := make([][]RGB, ph)
	in := make([][]bool, ph)
	for y := 0; y < ph; y++ {
		px[y] = make([]RGB, cols)
		in[y] = make([]bool, cols)
		for x := 0; x < cols; x++ {
			sx0, sx1 := x0+x*side/cols, x0+(x+1)*side/cols
			sy0, sy1 := y0+y*side/ph, y0+(y+1)*side/ph
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			if sy1 <= sy0 {
				sy1 = sy0 + 1
			}
			var r, g, bl, n uint64
			// 大きい画像でも重くならないよう、ブロック内は最大4×4点だけ平均する
			stepX, stepY := max(1, (sx1-sx0)/4), max(1, (sy1-sy0)/4)
			for sy := sy0; sy < sy1; sy += stepY {
				for sx := sx0; sx < sx1; sx += stepX {
					cr, cg, cb, _ := img.At(sx, sy).RGBA()
					r += uint64(cr)
					g += uint64(cg)
					bl += uint64(cb)
					n++
				}
			}
			px[y][x] = RGB{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8)}
			in[y][x] = true
			if round {
				// 1ピクセルの縦横比は1:1(1文字=横1×縦2ピクセル、文字は縦長なのでほぼ正方形)
				cx, cy := float64(x)+0.5-float64(cols)/2, (float64(y)+0.5-float64(ph)/2)*float64(cols)/float64(ph)
				rr := float64(cols) / 2
				in[y][x] = cx*cx+cy*cy <= rr*rr
			}
		}
	}
	out := make([]string, rows)
	for row := 0; row < rows; row++ {
		var sb strings.Builder
		for x := 0; x < cols; x++ {
			top, bot := px[row*2][x], px[row*2+1][x]
			ti, bi := in[row*2][x], in[row*2+1][x]
			switch {
			case ti && bi:
				sb.WriteString(fg(top) + bg(bot) + "▀")
			case ti:
				sb.WriteString("\x1b[49m" + fg(top) + "▀")
			case bi:
				sb.WriteString("\x1b[49m" + fg(bot) + "▄")
			default:
				sb.WriteString("\x1b[0m ")
			}
		}
		sb.WriteString("\x1b[0m")
		out[row] = sb.String()
	}
	return out
}
