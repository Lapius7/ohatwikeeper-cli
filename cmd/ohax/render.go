package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// 各画面の描画。データは公開APIから取り、表示はすべてこのCLIで組み立てる。

var jst = time.FixedZone("JST", 9*3600)

const avatarW = 20 // アバターの幅(文字数)。高さはその半分の行数で正方形になる

type view struct {
	uuid      string
	w         int
	images    bool // サムネイル・アバターを描くか
	limit     int  // gallery の件数
	weeks     int  // grass の週数
	days      int  // graph の期間
	isDefault bool // 既定ユーザーなら案内のコマンドでUUIDを省略する
}

// cmd は案内用のコマンド文字列。既定ユーザーならUUIDを省く
func (v view) cmd(sub string, extra ...string) string {
	parts := []string{"ohax"}
	if sub != "" {
		parts = append(parts, sub)
	}
	parts = append(parts, extra...)
	if !v.isDefault {
		parts = append(parts, v.uuid)
	}
	return strings.Join(parts, " ")
}

func (v view) profileCmd() string {
	if v.isDefault {
		return "ohax profile"
	}
	return "ohax " + v.uuid
}

func parseDay(s string) (time.Time, bool) {
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.ParseInLocation("2006-01-02", s, jst)
	return t, err == nil
}

func today() time.Time {
	n := time.Now().In(jst)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, jst)
}

func daysBetween(a, b time.Time) int { return int(math.Round(b.Sub(a).Hours() / 24)) }

func metricLine(r apiRecord) string {
	return paint(cPink, "♥ "+fmtInt(r.Likes.Int())) + "  " +
		paint(cGreen, "↻ "+fmtInt(r.Retweets.Int())) + "  " +
		paint(cViolet, "💬 "+fmtInt(r.Replies.Int())) + "  " +
		paint(cSky, "表示 "+fmtCompact(r.Views.Int()))
}

// ─────────────────────────── プロフィール ───────────────────────────

func renderProfile(v view) error {
	var (
		u      apiUser
		st     apiStats
		cal    map[string]calDay
		recent apiRecords
		avatar []string
	)
	sp := startSpinner("プロフィールを読み込み中…")
	err := parallel(
		func() (e error) {
			if u, e = getUser(v.uuid); e == nil && v.images && u.AvatarURL != "" {
				if img, ie := fetchImage(u.AvatarURL); ie == nil {
					avatar = halfBlock(img, avatarW, avatarW/2, true)
				}
			}
			return
		},
		func() (e error) { st, e = getStats(v.uuid); return },
		func() (e error) { cal, e = getCalendar(v.uuid); return },
		func() (e error) { recent, e = getRecords(v.uuid, 3, 0); return },
	)
	sp.Stop()
	if err != nil {
		return err
	}
	w := v.w
	out := banner(u.name(), u.XUsername, "プロフィール", w)

	// 名前・自己紹介。アバターがあれば左に並べる
	infoW := w - 3
	if avatar != nil {
		infoW = w - 3 - avatarW - 3
	}
	var info []string
	info = append(info, bold(gradientText(u.name(), []RGB{cSun, cOrange, cPink, cViolet})))
	sub := []string{}
	if u.XUsername != "" {
		sub = append(sub, link("https://x.com/"+u.XUsername, paint(cSky, "@"+u.XUsername)))
	}
	sub = append(sub, dim("ID "+u.PublicUUID))
	info = append(info, strings.Join(sub, dim("  ·  ")), "")
	bio := wrapText(u.Description, infoW)
	if len(bio) > 5 {
		bio = append(bio[:4], truncate(bio[4], infoW-1)+"…")
	}
	info = append(info, bio...)
	if len(bio) > 0 {
		info = append(info, "")
	}
	info = append(info, bold(fmtInt(u.Followers.Int()))+dim(" フォロワー")+"   "+bold(fmtInt(u.Following.Int()))+dim(" フォロー"))
	if url, label := u.websiteURL(); url != "" {
		info = append(info, paint(cFaint, "🔗 ")+link(url, paint(cSky, truncate(label, infoW-3))))
	}
	if u.Location != "" {
		info = append(info, paint(cFaint, "📍 ")+truncate(u.Location, infoW-3))
	}
	if avatar != nil {
		for i := 0; i < max(len(avatar), len(info)); i++ {
			a := strings.Repeat(" ", avatarW)
			if i < len(avatar) {
				a = avatar[i]
			}
			l := ""
			if i < len(info) {
				l = info[i]
			}
			out = append(out, "  "+a+"   "+l)
		}
	} else {
		for _, l := range info {
			out = append(out, "  "+l)
		}
	}
	out = append(out, "")

	// 統計タイル
	total := st.TotalRecords.Int()
	avg := func(sum int) float64 {
		if total == 0 {
			return 0
		}
		return float64(sum) / float64(total)
	}
	first, hasFirst := parseDay(st.FirstPost)
	since, dayN := "—", ""
	if hasFirst {
		since = first.Format("2006/01/02")
		dayN = fmtInt(daysBetween(first, today())+1) + "日目"
	}
	per := 3
	if w < 72 {
		per = 2
	}
	out = append(out, section("◆", "統計", w))
	out = append(out, tiles([]tile{
		{"投稿数", fmtInt(total) + "件", fmt.Sprintf("投稿率 %.1f%%", float64(st.PostRate)), cSun},
		{"総いいね", fmtCompact(st.TotalLikes.Int()), fmt.Sprintf("平均 %.1f", avg(st.TotalLikes.Int())), cPink},
		{"総インプレッション", fmtCompact(st.TotalViews.Int()), "平均 " + fmtCompact(int(avg(st.TotalViews.Int()))), cSky},
		{"総リポスト", fmtCompact(st.TotalReposts.Int()), fmt.Sprintf("平均 %.1f", avg(st.TotalReposts.Int())), cGreen},
		{"連続投稿", fmtInt(st.CurrentStreak.Int()) + "日", "最長 " + fmtInt(st.MaxStreak.Int()) + "日", cOrange},
		{"記録開始", since, dayN, cViolet},
	}, w, per)...)
	out = append(out, "")

	// 連続投稿・投稿率のバー
	cur, best := st.CurrentStreak.Int(), st.MaxStreak.Int()
	barW := max(10, w-36)
	ratio := 0.0
	if best > 0 {
		ratio = float64(cur) / float64(best)
	}
	streak := "  " + padRight(paint(cOrange, "🔥 連続投稿"), 12) + " " + bar(ratio, barW, []RGB{cSun, cOrange, cRed}) + "  " + boldPaint(cOrange, fmtInt(cur)+"日") + dim(" / 最長 "+fmtInt(best)+"日")
	if cur > 0 && cur >= best {
		streak += " " + paint(cSun, "★")
	}
	out = append(out, streak)
	out = append(out, "  "+padRight(paint(cSun, "📅 投稿率"), 12)+" "+bar(float64(st.PostRate)/100, barW, []RGB{cSun, cGreen})+"  "+boldPaint(cSun, fmt.Sprintf("%.1f%%", float64(st.PostRate))), "")

	// 直近30日のミニグラフ
	end := today()
	likes, views := make([]float64, 30), make([]float64, 30)
	for i := 0; i < 30; i++ {
		d := end.AddDate(0, 0, i-29).Format("2006-01-02")
		if c, ok := cal[d]; ok && c.Count > 0 {
			likes[i], views[i] = float64(c.Likes), float64(c.Views)
		} else {
			likes[i], views[i] = -1, -1
		}
	}
	maxOf := func(vs []float64) int {
		m := 0.0
		for _, x := range vs {
			m = math.Max(m, x)
		}
		return int(m)
	}
	out = append(out, section("◆", "直近30日", w))
	out = append(out, "  "+padRight(paint(cPink, "♥ いいね"), 12)+" "+sparkline(likes, []RGB{hex("#9D174D"), cPink})+"  "+dim("最高 ")+fmtInt(maxOf(likes)))
	out = append(out, "  "+padRight(paint(cSky, "表示"), 12)+" "+sparkline(views, []RGB{hex("#075985"), cSky})+"  "+dim("最高 ")+fmtCompact(maxOf(views)))
	out = append(out, "  "+strings.Repeat(" ", 13)+dim(padRight(end.AddDate(0, 0, -29).Format("1/2"), 25)+padLeft(end.Format("1/2"), 5)), "")

	// 最近の投稿
	if len(recent.Records) > 0 {
		out = append(out, section("◆", "最近の投稿", w))
		for _, r := range recent.Records {
			when := r.Date
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", r.Date, jst); err == nil {
				when = t.Format("2006/01/02 15:04")
			}
			out = append(out, "  "+paint(cSun, "●")+" "+dim(when)+"   "+metricLine(r))
			text := truncate(firstLine(r.Text), w-8)
			if text == "" {
				text = "(本文なし)"
			}
			out = append(out, "    "+link(r.URL, text))
		}
		out = append(out, "")
	}

	items := [][2]string{
		{v.cmd("graph"), "推移グラフ"},
		{v.cmd("grass"), "投稿グラス"},
		{v.cmd("awards"), "アワード"},
		{v.cmd("gallery"), "ギャラリー"},
		{v.cmd("open"), "ブラウザで開く"},
	}
	if !v.isDefault {
		items = append(items, [2]string{"ohax use " + v.uuid, "既定ユーザーにする"})
	}
	out = append(out, hints(items)...)
	out = append(out, "")
	printLines(out)
	return nil
}

// ─────────────────────────── 推移グラフ ───────────────────────────

var vBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func renderGraph(v view) error {
	var (
		u    apiUser
		recs []apiRecord
	)
	sp := startSpinner("投稿データを読み込み中…")
	err := parallel(
		func() (e error) { u, e = getUser(v.uuid); return },
		func() (e error) { recs, e = getAllRecords(v.uuid); return },
	)
	sp.Stop()
	if err != nil {
		return err
	}
	w := v.w
	out := banner(u.name(), u.XUsername, "推移グラフ", w)
	if len(recs) == 0 {
		out = append(out, "  "+dim("まだ投稿がありません"), "")
		printLines(out)
		return nil
	}

	start, _ := parseDay(recs[0].day())
	end, _ := parseDay(recs[len(recs)-1].day())
	if v.days > 0 {
		if s := end.AddDate(0, 0, -(v.days - 1)); s.After(start) {
			start = s
		}
	}
	var in []apiRecord
	for _, r := range recs {
		if d, ok := parseDay(r.day()); ok && !d.Before(start) {
			in = append(in, r)
		}
	}
	totalDays := daysBetween(start, end) + 1
	const axisW = 8
	plotW := w - axisW - 4
	cols := min(plotW, totalDays)
	perCol := float64(totalDays) / float64(cols)
	const h = 5

	metrics := []struct {
		name, icon string
		color      RGB
		get        func(apiRecord) float64
	}{
		{"いいね", "♥", cPink, func(r apiRecord) float64 { return float64(r.Likes) }},
		{"インプレッション", "◉", cSky, func(r apiRecord) float64 { return float64(r.Views) }},
		{"リポスト", "↻", cGreen, func(r apiRecord) float64 { return float64(r.Retweets) }},
		{"返信", "💬", cViolet, func(r apiRecord) float64 { return float64(r.Replies) }},
	}

	periodLabel := fmt.Sprintf("%s 〜 %s  (%s日間・%s件)", start.Format("2006/01/02"), end.Format("2006/01/02"), fmtInt(totalDays), fmtInt(len(in)))
	out = append(out, "  "+dim(periodLabel), "")

	for _, m := range metrics {
		sums, counts := make([]float64, cols), make([]int, cols)
		total, best := 0.0, 0.0
		bestDay := ""
		for _, r := range in {
			d, _ := parseDay(r.day())
			c := min(cols-1, int(float64(daysBetween(start, d))/perCol))
			x := m.get(r)
			sums[c] += x
			counts[c]++
			total += x
			if x > best {
				best, bestDay = x, d.Format("2006/01/02")
			}
		}
		vals := make([]float64, cols)
		maxV := 0.0
		for i := range vals {
			vals[i] = -1
			if counts[i] > 0 {
				vals[i] = sums[i] / float64(counts[i])
				maxV = math.Max(maxV, vals[i])
			}
		}

		// 直近30日と、その前の30日の1投稿あたり平均を比べる
		trend := ""
		var a, b float64
		var na, nb int
		for _, r := range recs {
			d, _ := parseDay(r.day())
			switch ago := daysBetween(d, end); {
			case ago < 30:
				a += m.get(r)
				na++
			case ago < 60:
				b += m.get(r)
				nb++
			}
		}
		if na > 0 && nb > 0 && b > 0 {
			pct := (a/float64(na) - b/float64(nb)) / (b / float64(nb)) * 100
			switch {
			case pct >= 0.5:
				trend = paint(cGreen, fmt.Sprintf("▲ %.0f%%", pct))
			case pct <= -0.5:
				trend = paint(cRed, fmt.Sprintf("▼ %.0f%%", -pct))
			default:
				trend = dim("→ 横ばい")
			}
			trend += dim(" 直近30日")
		}

		avgAll := 0.0
		if len(in) > 0 {
			avgAll = total / float64(len(in))
		}
		head := " " + boldPaint(m.color, m.icon+" "+m.name) + "   " +
			dim("平均 ") + bold(fmtAvg(avgAll)) + "   " +
			dim("合計 ") + bold(fmtCompact(int(total))) + "   " +
			dim("最高 ") + bold(fmtInt(int(best))) + dim(" ("+bestDay+")")
		if trend != "" && strWidth(head)+3+strWidth(trend) <= w {
			head += "   " + trend
		}
		out = append(out, head)

		dark := lerp(m.color, RGB{}, 0.55)
		for row := 0; row < h; row++ {
			label := ""
			switch row {
			case 0:
				label = fmtCompact(int(math.Round(maxV)))
			case h - 1:
				label = "0"
			}
			var sb strings.Builder
			sb.WriteString(" " + dim(padLeft(label, axisW)) + " " + paint(cFaint, "┤") + fg(gradientAt([]RGB{dark, m.color}, float64(h-1-row)/float64(h-1))))
			for c := 0; c < cols; c++ {
				if vals[c] < 0 || maxV == 0 {
					sb.WriteString(" ")
					continue
				}
				level := vals[c] / maxV * float64(h*8)
				fill := level - float64((h-1-row)*8)
				switch {
				case fill >= 8:
					sb.WriteString("█")
				case fill >= 1:
					sb.WriteString(vBlocks[int(fill)])
				case row == h-1:
					sb.WriteString("▁") // 投稿がある列は最低限見えるように
				default:
					sb.WriteString(" ")
				}
			}
			sb.WriteString(reset())
			out = append(out, sb.String())
		}
		out = append(out, " "+strings.Repeat(" ", axisW+1)+paint(cFaint, "└"+strings.Repeat("─", cols)))
		l, r := start.Format("2006/01"), end.Format("2006/01")
		if cols > 30 {
			mid := start.AddDate(0, 0, totalDays/2).Format("2006/01")
			left := padRight(l, cols/2-strWidth(mid)/2)
			out = append(out, " "+strings.Repeat(" ", axisW+2)+dim(left+padRight(mid, cols-strWidth(left)-strWidth(r))+r))
		} else {
			out = append(out, " "+strings.Repeat(" ", axisW+2)+dim(padRight(l, cols-strWidth(r))+r))
		}
		out = append(out, "")
	}

	unit := "1列 = 1日"
	if perCol > 1.05 {
		unit = fmt.Sprintf("1列 = 約%s日", trimZero(perCol))
	}
	out = append(out, "  "+dim(unit+" · 棒の高さはその期間の1投稿あたり平均"), "")
	out = append(out, hints([][2]string{
		{v.cmd("graph", "--days", "90"), "直近90日だけ表示"},
		{v.cmd("grass"), "投稿グラス"},
		{v.cmd("open", "graph"), "ブラウザで詳しく見る"},
	})...)
	out = append(out, "")
	printLines(out)
	return nil
}

func fmtAvg(f float64) string {
	if f >= 10000 {
		return fmtCompact(int(f))
	}
	if f >= 100 {
		return fmtInt(int(math.Round(f)))
	}
	return trimZero(f)
}

// ─────────────────────────── 投稿グラス ───────────────────────────

var grassColors = []RGB{cEmpty, hex("#9A3412"), hex("#EA580C"), hex("#FB923C"), hex("#FDE68A")}
var grassPlain = []string{"·", "░", "▒", "▓", "█"}

func grassCell(level int) string {
	if colorLevel == 0 {
		return grassPlain[level]
	}
	return paint(grassColors[level], "■")
}

func renderGrass(v view) error {
	var (
		u   apiUser
		st  apiStats
		cal map[string]calDay
	)
	sp := startSpinner("投稿カレンダーを読み込み中…")
	err := parallel(
		func() (e error) { u, e = getUser(v.uuid); return },
		func() (e error) { st, e = getStats(v.uuid); return },
		func() (e error) { cal, e = getCalendar(v.uuid); return },
	)
	sp.Stop()
	if err != nil {
		return err
	}
	w := v.w
	weeks := v.weeks
	if weeks <= 0 {
		weeks = min(53, (w-6)/2)
	}
	weeks = max(4, weeks)
	out := banner(u.name(), u.XUsername, fmt.Sprintf("投稿グラス・%d週間", weeks), w)

	now := today()
	start := now.AddDate(0, 0, -int(now.Weekday())-7*(weeks-1)) // 日曜始まり
	span := daysBetween(start, now) + 1

	// 濃さ: 期間中に投稿した日のいいね数を4段階に分ける
	var likes []float64
	posted, bestLikes := 0, 0
	bestDay := ""
	for i := 0; i < span; i++ {
		d := start.AddDate(0, 0, i)
		if c, ok := cal[d.Format("2006-01-02")]; ok && c.Count > 0 {
			posted++
			likes = append(likes, float64(c.Likes))
			if c.Likes.Int() > bestLikes {
				bestLikes, bestDay = c.Likes.Int(), d.Format("2006/01/02")
			}
		}
	}
	sort.Float64s(likes)
	q := func(p float64) float64 {
		if len(likes) == 0 {
			return 0
		}
		return likes[min(len(likes)-1, int(p*float64(len(likes))))]
	}
	q1, q2, q3 := q(0.25), q(0.5), q(0.75)
	level := func(d time.Time) int {
		c, ok := cal[d.Format("2006-01-02")]
		if !ok || c.Count == 0 {
			return 0
		}
		switch l := float64(c.Likes); {
		case l >= q3:
			return 4
		case l >= q2:
			return 3
		case l >= q1:
			return 2
		}
		return 1
	}

	// 月の見出し(その月の1日を含む週の列に置く。先頭列は次の月まで余裕があるときだけ)
	const labelW = 5
	var mb strings.Builder
	col := -1
	for wk := 0; wk < weeks; wk++ {
		sat := start.AddDate(0, 0, 7*wk+6)
		label := ""
		if sat.Day() <= 7 {
			label = fmt.Sprintf("%d月", sat.Month())
		} else if wk == 0 && sat.Day() <= 14 {
			label = fmt.Sprintf("%d月", start.Month())
		}
		pos := wk * 2
		if label == "" || pos <= col || pos+strWidth(label) > weeks*2 {
			continue
		}
		mb.WriteString(strings.Repeat(" ", pos-max(col, 0)) + label)
		col = pos + strWidth(label)
	}
	out = append(out, "  "+dim(fmt.Sprintf("%s 〜 %s", start.Format("2006/01/02"), now.Format("2006/01/02"))), "")
	out = append(out, strings.Repeat(" ", labelW)+dim(mb.String()))

	wdays := []string{"日", "月", "火", "水", "木", "金", "土"}
	for wd := 0; wd < 7; wd++ {
		var sb strings.Builder
		sb.WriteString("  " + dim(wdays[wd]) + " ")
		for wk := 0; wk < weeks; wk++ {
			d := start.AddDate(0, 0, 7*wk+wd)
			if d.After(now) {
				sb.WriteString("  ")
				continue
			}
			sb.WriteString(grassCell(level(d)) + " ")
		}
		out = append(out, sb.String())
	}
	legend := "  " + strings.Repeat(" ", 3) + dim("少ない ")
	for i := range grassColors {
		legend += grassCell(i) + " "
	}
	legend += dim("多い") + "   " + dim("色の濃さ = その日のいいね数")
	out = append(out, "", legend, "")

	per := 4
	if w < 80 {
		per = 2
	}
	best := "—"
	if bestDay != "" {
		best = "♥ " + fmtInt(bestLikes)
	}
	out = append(out, tiles([]tile{
		{"投稿した日", fmt.Sprintf("%s / %s日", fmtInt(posted), fmtInt(span)), fmt.Sprintf("%.1f%%", float64(posted)/float64(span)*100), cSun},
		{"連続投稿", fmtInt(st.CurrentStreak.Int()) + "日", "最長 " + fmtInt(st.MaxStreak.Int()) + "日", cOrange},
		{"ベストの日", best, bestDay, cPink},
		{"通算", fmtInt(st.TotalRecords.Int()) + "件", fmt.Sprintf("投稿率 %.1f%%", float64(st.PostRate)), cViolet},
	}, w, per)...)
	out = append(out, "")
	out = append(out, hints([][2]string{
		{v.cmd("grass", "--weeks", "12"), "直近12週間だけ表示"},
		{v.cmd("graph"), "推移グラフ"},
		{v.cmd("awards"), "アワード"},
	})...)
	out = append(out, "")
	printLines(out)
	return nil
}

// ─────────────────────────── アワード ───────────────────────────

var (
	streakMilestones = []int{3, 7, 14, 21, 30, 50, 100, 150, 200, 300, 365, 500, 730, 1000, 1095, 1500, 1825, 2555, 3650}
	postMilestones   = []int{5, 10, 25, 50, 75, 100, 150, 200, 300, 500, 750, 1000, 1500, 2000, 2500, 3000, 4000, 5000, 7500, 10000}
	likeMilestones   = []int{50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000, 250000, 500000, 1000000}
	tierColors       = []RGB{hex("#CD7F32"), hex("#D4D4D8"), hex("#FACC15"), hex("#5EEAD4"), hex("#A78BFA"), hex("#F472B6")}
)

func streakLabel(t int) string {
	if t >= 365 && t%365 == 0 {
		return fmt.Sprintf("%d年", t/365)
	}
	return fmt.Sprintf("%d日", t)
}

func chip(text string, c RGB) string {
	if colorLevel == 0 {
		return "[" + text + "]"
	}
	return bg(c) + fg(hex("#18181B")) + sgr("1") + " " + text + " " + reset()
}

func renderAwards(v view) error {
	var (
		u  apiUser
		aw apiAwards
	)
	sp := startSpinner("アワードを読み込み中…")
	err := parallel(
		func() (e error) { u, e = getUser(v.uuid); return },
		func() (e error) { aw, e = getAwards(v.uuid); return },
	)
	sp.Stop()
	if err != nil {
		return err
	}
	w := v.w
	out := banner(u.name(), u.XUsername, "アワード", w)

	cats := []struct {
		key, title, icon string
		th               []int
		label            func(int) string
		unit             func(int) string
		color            RGB
	}{
		{"streak", "連続投稿", "🔥", streakMilestones, streakLabel, func(n int) string { return fmtInt(n) + "日" }, cOrange},
		{"posts", "投稿数", "📝", postMilestones, func(t int) string { return fmtInt(t) + "回" }, func(n int) string { return fmtInt(n) + "回" }, cSun},
		{"likes", "いいね", "💖", likeMilestones, func(t int) string { return fmtCompact(t) }, func(n int) string { return fmtInt(n) }, cPink},
	}
	got := map[string]map[int]bool{}
	for _, a := range aw.Awards {
		if got[a.Category] == nil {
			got[a.Category] = map[int]bool{}
		}
		got[a.Category][a.Threshold.Int()] = true
	}
	allN := len(streakMilestones) + len(postMilestones) + len(likeMilestones)

	per := 4
	if w < 80 {
		per = 2
	}
	out = append(out, tiles([]tile{
		{"獲得アワード", fmt.Sprintf("%d個", len(aw.Awards)), fmt.Sprintf("全%d個中", allN), cSun},
		{"最長連続", fmtInt(aw.Stats.MaxStreak.Int()) + "日", "いま " + fmtInt(aw.Stats.CurrentStreak.Int()) + "日", cOrange},
		{"投稿数", fmtInt(aw.Stats.TotalPosts.Int()) + "件", "", cViolet},
		{"総いいね", fmtCompact(aw.Stats.TotalLikes.Int()), "", cPink},
	}, w, per)...)
	out = append(out, "")

	for _, c := range cats {
		n := len(got[c.key])
		out = append(out, section(c.icon, c.title+"  "+dim(fmt.Sprintf("%d / %d", n, len(c.th))), w))
		line := "  "
		var lines []string
		for i, t := range c.th {
			if !got[c.key][t] {
				continue
			}
			ch := chip(c.label(t), tierColors[i*len(tierColors)/len(c.th)])
			if strWidth(line)+strWidth(ch)+1 > w {
				lines = append(lines, line)
				line = "  "
			}
			line += ch + " "
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			lines = append(lines, "  "+dim("まだありません"))
		}
		out = append(out, lines...)

		for _, m := range aw.Milestones {
			if m.Category != c.key {
				continue
			}
			pct := math.Min(1, float64(m.Progress))
			head := "  " + dim("次の目標 ") + boldPaint(c.color, c.label(m.Threshold.Int())) + dim("  あと ") + bold(c.unit(m.Remaining.Int()))
			barW := max(10, w-strWidth(head)-10)
			out = append(out, head+"  "+bar(pct, barW, []RGB{c.color, lerp(c.color, hex("#FFFFFF"), 0.4)})+" "+dim(fmt.Sprintf("%.0f%%", pct*100)))
		}
		if n == len(c.th) {
			out = append(out, "  "+paint(cSun, "★ すべて達成！"))
		}
		out = append(out, "")
	}
	out = append(out, hints([][2]string{
		{v.profileCmd(), "プロフィール"},
		{v.cmd("grass"), "投稿グラス"},
		{v.cmd("open", "awards"), "ブラウザで全実績を見る"},
	})...)
	out = append(out, "")
	printLines(out)
	return nil
}

// ─────────────────────────── ギャラリー ───────────────────────────

func renderGallery(v view) error {
	var u apiUser
	var items []apiRecord
	sp := startSpinner("ギャラリーを読み込み中…")
	err := parallel(
		func() (e error) { u, e = getUser(v.uuid); return },
		func() error {
			for off := 0; off < 1000 && len(items) < v.limit; off += 100 {
				r, e := getRecords(v.uuid, 100, off)
				if e != nil {
					return e
				}
				for _, rec := range r.Records {
					if rec.ImageURL != "" && len(items) < v.limit {
						items = append(items, rec)
					}
				}
				if len(r.Records) < 100 {
					break
				}
			}
			return nil
		},
	)
	var thumbs [][]string
	w := v.w
	cols := max(1, (w-2+2)/26)
	tw := min(30, (w-2-(cols-1)*2)/cols)
	th := tw / 2
	if err == nil && v.images {
		thumbs = make([][]string, len(items))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, it := range items {
			wg.Add(1)
			go func(i int, u string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if img, e := fetchImage(twimgSmall(u, "thumb")); e == nil {
					thumbs[i] = halfBlock(img, tw, th, false)
				}
			}(i, it.ImageURL)
		}
		wg.Wait()
	}
	sp.Stop()
	if err != nil {
		return err
	}
	out := banner(u.name(), u.XUsername, "ギャラリー", w)
	if len(items) == 0 {
		out = append(out, "  "+dim("画像つきの投稿はまだありません"), "")
		printLines(out)
		return nil
	}
	open := func(r apiRecord) string {
		u := r.ImageProxyURL
		if u == "" {
			u = r.ImageURL
		}
		return u
	}

	if v.images {
		for i := 0; i < len(items); i += cols {
			row := items[i:min(i+cols, len(items))]
			for y := 0; y < th; y++ {
				line := " "
				for j := range row {
					cell := ""
					if t := thumbs[i+j]; t != nil {
						cell = t[y]
					} else if y == th/2 {
						cell = padRight(strings.Repeat(" ", max(0, tw/2-4))+dim("画像なし"), tw)
					} else {
						cell = strings.Repeat(" ", tw)
					}
					line += " " + cell + " "
				}
				out = append(out, line)
			}
			caps := make([]string, 4)
			for _, r := range row {
				date := boldPaint(cSun, strings.ReplaceAll(r.day(), "-", "/"))
				if r.VideoURL != "" {
					date += " " + paint(cViolet, "▶動画")
				}
				caps[0] += "  " + padRight(date, tw)
				caps[1] += "  " + padRight(paint(cPink, "♥ "+fmtInt(r.Likes.Int()))+"  "+paint(cGreen, "↻ "+fmtInt(r.Retweets.Int()))+"  "+paint(cSky, "表示 "+fmtCompact(r.Views.Int())), tw)
				caps[2] += "  " + padRight(dim(truncate(firstLine(r.Text), tw)), tw)
				caps[3] += "  " + padRight(link(open(r), paint(cSky, "画像を開く ↗")), tw)
			}
			out = append(out, caps...)
			out = append(out, "")
		}
	} else {
		for i, r := range items {
			branch := "├"
			if i == len(items)-1 {
				branch = "└"
			}
			head := "  " + paint(cFaint, branch) + " " + boldPaint(cSun, strings.ReplaceAll(r.day(), "-", "/")) + "  " + paint(cPink, padLeft("♥ "+fmtInt(r.Likes.Int()), 8)) + "  "
			if linksOn {
				rest := w - strWidth(head) - 14
				out = append(out, head+dim(padRight(truncate(firstLine(r.Text), rest), rest))+"  "+link(open(r), paint(cSky, "画像を開く ↗")))
			} else {
				out = append(out, head+truncate(firstLine(r.Text), w-strWidth(head)))
				pipe := "│"
				if i == len(items)-1 {
					pipe = " "
				}
				out = append(out, "  "+pipe+"   "+open(r))
			}
		}
		out = append(out, "")
	}
	out = append(out, "  "+dim(fmt.Sprintf("画像つきの投稿を新しい順に%d件表示しています", len(items))), "")
	out = append(out, hints([][2]string{
		{v.cmd("gallery", "--limit", "30"), "もっと表示"},
		{v.cmd("open", "gallery"), "ブラウザで全画像を見る"},
	})...)
	out = append(out, "")
	printLines(out)
	return nil
}
