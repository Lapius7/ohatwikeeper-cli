package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// 端末出力まわり: 色(truecolor/256色/なし)、OSC8リンク、日本語を含む表示幅の計算。

type RGB struct{ R, G, B uint8 }

var (
	colorLevel int  // 0: 色なし 1: 256色 2: truecolor
	linksOn    bool // OSC8ハイパーリンクを出すか
)

func hex(s string) RGB {
	v, _ := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

// 朝焼けをイメージした配色
var (
	cSun    = hex("#FBBF24")
	cOrange = hex("#FB923C")
	cPink   = hex("#F472B6")
	cViolet = hex("#A78BFA")
	cSky    = hex("#38BDF8")
	cGreen  = hex("#34D399")
	cRed    = hex("#F87171")
	cFaint  = hex("#6B7280")
	cEmpty  = hex("#3F3F46")

	sunrise = []RGB{cSun, cOrange, cPink, cViolet, cSky}
)

// setupColor は --color/--no-color の指定(force: 1=強制オン, -1=強制オフ, 0=自動)から色の段階を決める
func setupColor(force int) {
	enableVT()
	tty := isTTY(os.Stdout)
	switch {
	case force < 0:
		colorLevel = 0
	case force == 0 && (!tty || os.Getenv("NO_COLOR") != ""):
		colorLevel = 0
	default:
		colorLevel = detectColorDepth()
	}
	if v := strings.ToLower(os.Getenv("OHAX_COLOR")); v != "" && force >= 0 {
		switch v {
		case "truecolor", "24bit":
			colorLevel = 2
		case "256":
			colorLevel = 1
		case "none", "0", "false":
			colorLevel = 0
		}
	}
	linksOn = colorLevel > 0
}

func detectColorDepth() int {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return 2
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "vscode", "WezTerm", "ghostty", "Hyper":
		return 2
	}
	if os.Getenv("WT_SESSION") != "" {
		return 2
	}
	return 1
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// termWidth は描画に使う幅。COLUMNS → 端末サイズ → 80。読みやすさのため100で頭打ち
func termWidth(override int) int {
	w := override
	if w <= 0 {
		if v, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && v > 0 {
			w = v
		} else if tw := ttyWidth(); tw > 0 {
			w = tw
		} else {
			w = 80
		}
		if w > 100 {
			w = 100
		}
	}
	if w < 40 {
		w = 40
	}
	return w - 1 // 右端ぴったりで折り返す端末があるので1桁余らせる
}

func to256(c RGB) int {
	if c.R == c.G && c.G == c.B {
		if c.R < 8 {
			return 16
		}
		if c.R > 238 {
			return 231
		}
		return 232 + (int(c.R)-8)/10
	}
	q := func(v uint8) int {
		if v < 48 {
			return 0
		}
		if v < 115 {
			return 1
		}
		return (int(v) - 35) / 40
	}
	return 16 + 36*q(c.R) + 6*q(c.G) + q(c.B)
}

func fg(c RGB) string {
	switch colorLevel {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("\x1b[38;5;%dm", to256(c))
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func bg(c RGB) string {
	switch colorLevel {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("\x1b[48;5;%dm", to256(c))
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

func sgr(code string) string {
	if colorLevel == 0 {
		return ""
	}
	return "\x1b[" + code + "m"
}

func reset() string { return sgr("0") }

func paint(c RGB, s string) string {
	if colorLevel == 0 || s == "" {
		return s
	}
	return fg(c) + s + reset()
}

func bold(s string) string   { return sgr("1") + s + sgr("22") }
func dim(s string) string    { return sgr("2") + s + sgr("22") }
func italic(s string) string { return sgr("3") + s + sgr("23") }

func boldPaint(c RGB, s string) string {
	if colorLevel == 0 {
		return s
	}
	return sgr("1") + fg(c) + s + reset()
}

func lerp(a, b RGB, t float64) RGB {
	t = math.Max(0, math.Min(1, t))
	f := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return RGB{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B)}
}

func gradientAt(stops []RGB, t float64) RGB {
	if len(stops) == 1 {
		return stops[0]
	}
	t = math.Max(0, math.Min(1, t))
	pos := t * float64(len(stops)-1)
	i := int(pos)
	if i >= len(stops)-1 {
		return stops[len(stops)-1]
	}
	return lerp(stops[i], stops[i+1], pos-float64(i))
}

// gradientText は文字ごとに色を変える
func gradientText(s string, stops []RGB) string {
	if colorLevel == 0 {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		t := 0.0
		if len(rs) > 1 {
			t = float64(i) / float64(len(rs)-1)
		}
		b.WriteString(fg(gradientAt(stops, t)))
		b.WriteRune(r)
	}
	b.WriteString(reset())
	return b.String()
}

func link(url, text string) string {
	if !linksOn || url == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// ─── 表示幅 ───

func runeWidth(r rune) int {
	switch {
	case r == 0x200D || (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0x0300 && r <= 0x036F) || (r >= 0x1F3FB && r <= 0x1F3FF):
		return 0
	case r < 0x1100:
		return 1
	case r <= 0x115F,
		r >= 0x2E80 && r <= 0xA4CF && r != 0x303F,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1F64F,
		r >= 0x1F680 && r <= 0x1F6FF,
		r >= 0x1F900 && r <= 0x1FAFF,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

// strWidth はエスケープシーケンスを除いた表示幅
func strWidth(s string) int {
	n := 0
	for _, r := range stripANSI(s) {
		n += runeWidth(r)
	}
	return n
}

func padRight(s string, w int) string {
	if n := strWidth(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := strWidth(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// truncate はプレーンテキストを表示幅wに収める(はみ出したら…)
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if strWidth(s) <= w {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		rw := runeWidth(r)
		if n+rw > w-1 {
			break
		}
		b.WriteRune(r)
		n += rw
	}
	return b.String() + "…"
}

// wrapText はプレーンテキストを表示幅wで折り返す(改行は維持、空行は詰める)
func wrapText(s string, w int) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		var b strings.Builder
		n := 0
		for _, r := range para {
			rw := runeWidth(r)
			if n+rw > w {
				out = append(out, b.String())
				b.Reset()
				n = 0
			}
			b.WriteRune(r)
			n += rw
		}
		if b.Len() > 0 {
			out = append(out, b.String())
		}
	}
	return out
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// ─── 数値 ───

func fmtInt(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// fmtCompact は 12,345 → 1.2万、123,456,789 → 1.2億
func fmtCompact(n int) string {
	f := float64(n)
	switch {
	case f >= 1e8:
		return trimZero(f/1e8) + "億"
	case f >= 1e4:
		return trimZero(f/1e4) + "万"
	}
	return fmtInt(n)
}

func trimZero(f float64) string {
	if f >= 100 {
		return strconv.Itoa(int(math.Round(f)))
	}
	return strings.TrimSuffix(strconv.FormatFloat(f, 'f', 1, 64), ".0")
}

// ─── 部品 ───

var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// bar は比率ratioの横棒。色はグラデーション
func bar(ratio float64, width int, stops []RGB) string {
	ratio = math.Max(0, math.Min(1, ratio))
	if colorLevel == 0 {
		full := int(math.Round(ratio * float64(width)))
		return "[" + strings.Repeat("#", full) + strings.Repeat("-", width-full) + "]"
	}
	cells := ratio * float64(width)
	full := int(cells)
	part := int((cells - float64(full)) * 8)
	var b strings.Builder
	for i := 0; i < full; i++ {
		b.WriteString(fg(gradientAt(stops, float64(i)/math.Max(1, float64(width-1)))))
		b.WriteString("█")
	}
	used := full
	if part > 0 && full < width {
		b.WriteString(fg(gradientAt(stops, float64(full)/math.Max(1, float64(width-1)))))
		b.WriteString(eighths[part])
		used++
	}
	b.WriteString(fg(cEmpty))
	b.WriteString(strings.Repeat("░", width-used))
	b.WriteString(reset())
	return b.String()
}

var sparks = []rune("▁▂▃▄▅▆▇█")

// sparkline は値の列を1行のミニグラフにする(負の値は投稿なしとして「·」)。
// 変化が見えるよう、0からではなく最小〜最大の範囲で高さを決める
func sparkline(vals []float64, stops []RGB) string {
	lo, hi := math.Inf(1), 0.0
	for _, v := range vals {
		if v >= 0 {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	var b strings.Builder
	for i, v := range vals {
		if v < 0 || hi == 0 {
			b.WriteString(paint(cEmpty, "·"))
			continue
		}
		lv := 7
		if hi > lo {
			lv = 1 + int(math.Round((v-lo)/(hi-lo)*6))
		}
		b.WriteString(fg(gradientAt(stops, float64(i)/math.Max(1, float64(len(vals)-1)))))
		b.WriteRune(sparks[lv])
	}
	b.WriteString(reset())
	return b.String()
}

// section は「 ◆ 見出し ────」の区切り行
func section(icon, title string, w int) string {
	head := " " + paint(cSun, icon) + " " + bold(title) + " "
	rest := w - strWidth(head)
	if rest < 0 {
		rest = 0
	}
	return head + paint(cEmpty, strings.Repeat("─", rest))
}

// banner は各画面の先頭。ブランド+ユーザー名+ページ名と、朝焼けグラデーションの線
func banner(name, handle, page string, w int) []string {
	left := " " + gradientText("☀ ohax", sunrise) + "  " + bold(name)
	if handle != "" {
		left += " " + dim("@"+handle)
	}
	right := paint(cSky, page) + " "
	gap := w - strWidth(left) - strWidth(right)
	if gap < 1 {
		gap = 1
	}
	line := ""
	if colorLevel == 0 {
		line = " " + strings.Repeat("=", w-1)
	} else {
		line = " " + gradientText(strings.Repeat("━", w-1), sunrise)
	}
	return []string{"", left + strings.Repeat(" ", gap) + right, line, ""}
}

type tile struct {
	label, value, sub string
	color             RGB
}

// tiles は角丸の小さなカードを横に並べる
func tiles(ts []tile, w, perRow int) []string {
	var out []string
	gap := 1
	tw := (w - 1 - gap*(perRow-1)) / perRow
	inner := tw - 4
	border := func(s string) string { return paint(cFaint, s) }
	for i := 0; i < len(ts); i += perRow {
		row := ts[i:min(i+perRow, len(ts))]
		lines := make([]string, 4)
		for j, t := range row {
			sep := " "
			if j == 0 {
				sep = " "
			} else {
				sep = strings.Repeat(" ", gap)
			}
			label := truncate(t.label, tw-6) // 見出しの右に最低1本は罫線を残す
			top := border("╭─") + " " + dim(label) + " " + border(strings.Repeat("─", max(0, tw-5-strWidth(label)))+"╮")
			val := boldPaint(t.color, truncate(t.value, inner))
			sub := dim(truncate(t.sub, inner))
			lines[0] += sep + top
			lines[1] += sep + border("│") + " " + padRight(val, inner) + " " + border("│")
			lines[2] += sep + border("│") + " " + padRight(sub, inner) + " " + border("│")
			lines[3] += sep + border("╰"+strings.Repeat("─", tw-2)+"╯")
		}
		out = append(out, lines...)
	}
	return out
}

// hint はフッターのコマンド案内
func hints(items [][2]string) []string {
	cw := 0
	for _, it := range items {
		cw = max(cw, strWidth(it[0]))
	}
	var out []string
	for _, it := range items {
		out = append(out, "  "+paint(cOrange, "›")+" "+padRight(paint(cSky, it[0]), cw)+"  "+dim(it[1]))
	}
	return out
}

func printLines(lines []string) {
	fmt.Println(strings.Join(lines, "\n"))
}

// ─── 読み込み中表示 ───

type spinner struct {
	stop chan struct{}
	done chan struct{}
}

func startSpinner(msg string) *spinner {
	s := &spinner{stop: make(chan struct{}), done: make(chan struct{})}
	if colorLevel == 0 || !isTTY(os.Stderr) {
		close(s.done)
		return s
	}
	go func() {
		defer close(s.done)
		frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			fmt.Fprintf(os.Stderr, "\r %s %s", paint(gradientAt(sunrise, float64(i%20)/19), string(frames[i%len(frames)])), dim(msg))
			select {
			case <-s.stop:
				fmt.Fprint(os.Stderr, "\r\x1b[K")
				return
			case <-t.C:
			}
		}
	}()
	return s
}

func (s *spinner) Stop() {
	select {
	case <-s.done:
		return
	default:
	}
	close(s.stop)
	<-s.done
}
