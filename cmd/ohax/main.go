// ohax — おはツイKeeper(ohatwikeeper.com)の公開プロフィールをターミナルで見るCLI。
//
// サーバーはUser-Agentに"curl"を含むリクエストへANSIテキストのカードを返すので、
// このCLIはそれを取得して表示するだけの薄いクライアント。Go標準ライブラリのみ。
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var version = "dev"

const defaultTimeout = 15 * time.Second

// セクション名 → URLパス。プロフィールは空パス。
var sections = []struct{ name, path, label string }{
	{"profile", "", "プロフィール"},
	{"graph", "graph", "推移グラフ"},
	{"grass", "grass", "投稿グラス"},
	{"awards", "awards", "アワード"},
	{"gallery", "gallery", "ギャラリー"},
}

// rssは生XMLなので"all"には含めない
var extraSections = map[string]string{"rss": "rss"}

var uuidRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type options struct {
	color   bool
	timeout time.Duration
}

func main() {
	// go install で入れた場合は -ldflags が無いので、モジュールのバージョン(v0.1.0等)を使う
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts := options{color: defaultColor(), timeout: defaultTimeout}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-n", "--no-color", "--nocolor":
			opts.color = false
		case "--color":
			opts.color = true
		case "-h", "--help":
			usage(os.Stdout)
			return 0
		case "-v", "--version":
			fmt.Println("ohax", version)
			return 0
		case "--clear": // ohax use --clear
			pos = append(pos, a)
		case "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				fmt.Fprintf(os.Stderr, "ohax: 不明なオプション: %s\n", a)
				return 2
			}
			pos = append(pos, a)
		}
	}

	cmd := ""
	if len(pos) > 0 {
		cmd = pos[0]
	}
	switch cmd {
	case "help":
		usage(os.Stdout)
		return 0
	case "version":
		fmt.Println("ohax", version)
		return 0
	case "use":
		return cmdUse(pos[1:])
	case "whoami":
		return cmdWhoami()
	case "all":
		return withTarget(pos[1:], "", func(uuid, _ string) int { return cmdAll(uuid, opts) })
	case "open", "url":
		rest := pos[1:]
		section := ""
		if len(rest) > 0 && isSection(rest[0]) {
			section, rest = rest[0], rest[1:]
		}
		return withTarget(rest, section, func(uuid, sec string) int {
			u := browserURL(uuid, sec)
			if cmd == "url" {
				fmt.Println(u)
				return 0
			}
			if err := openBrowser(u); err != nil {
				fmt.Fprintf(os.Stderr, "ohax: ブラウザを開けませんでした: %v\n%s\n", err, u)
				return 1
			}
			return 0
		})
	}

	section := ""
	rest := pos
	if isSection(cmd) {
		section, rest = cmd, pos[1:]
	}
	return withTarget(rest, section, func(uuid, sec string) int {
		body, err := fetch(uuid, pathOf(sec), opts)
		if body != "" {
			fmt.Print(body)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ohax: %v\n", err)
			return 1
		}
		return 0
	})
}

// withTarget は引数(UUIDかURL)→環境変数→保存済み設定の順で対象ユーザーを決める。
// URLにセクションが含まれていて、コマンドでセクションが指定されていなければそれを使う。
func withTarget(rest []string, section string, fn func(uuid, section string) int) int {
	if len(rest) > 1 {
		fmt.Fprintf(os.Stderr, "ohax: 引数が多すぎます: %s\n", strings.Join(rest[1:], " "))
		return 2
	}
	var uuid string
	if len(rest) == 1 {
		u, sec, err := parseTarget(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ohax: %v\n", err)
			return 2
		}
		uuid = u
		if section == "" {
			section = sec
		}
	} else {
		uuid = defaultUUID()
		if uuid == "" {
			fmt.Fprintln(os.Stderr, "ohax: ユーザーが指定されていません。")
			fmt.Fprintln(os.Stderr, "  ohax <public_uuid> のように指定するか、ohax use <public_uuid> で既定ユーザーを保存してください。")
			return 2
		}
	}
	return fn(uuid, section)
}

// parseTarget は "5axwn" / "https://5axwn.ohax.pw/graph" / "ohax.pw/5axwn" /
// "ohatwikeeper.com/5axwn/awards" / "5axwn.ohatwikeeper.com" を受け付ける。
func parseTarget(s string) (uuid, section string, err error) {
	s = strings.TrimSpace(s)
	if uuidRe.MatchString(s) {
		return s, "", nil
	}
	t := s
	if i := strings.Index(t, "://"); i >= 0 {
		t = t[i+3:]
	}
	if i := strings.IndexAny(t, "?#"); i >= 0 {
		t = t[:i]
	}
	host, path, _ := strings.Cut(t, "/")
	host = strings.ToLower(host)
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })

	switch {
	case host == "ohax.pw" || host == "ohatwikeeper.com" || host == "www.ohatwikeeper.com":
		if len(parts) == 0 {
			break
		}
		uuid = parts[0]
		parts = parts[1:]
	case strings.HasSuffix(host, ".ohax.pw"):
		uuid = strings.TrimSuffix(host, ".ohax.pw")
	case strings.HasSuffix(host, ".ohatwikeeper.com"):
		uuid = strings.TrimSuffix(host, ".ohatwikeeper.com")
	}
	if uuid == "" || !uuidRe.MatchString(uuid) {
		return "", "", fmt.Errorf("public_uuidまたはおはツイKeeperのURLとして解釈できません: %s", s)
	}
	if len(parts) > 0 && isSection(parts[0]) {
		section = parts[0]
	}
	return uuid, section, nil
}

func isSection(s string) bool {
	if _, ok := extraSections[s]; ok {
		return true
	}
	for _, sec := range sections {
		if sec.name == s {
			return true
		}
	}
	return false
}

func pathOf(section string) string {
	if p, ok := extraSections[section]; ok {
		return p
	}
	for _, sec := range sections {
		if sec.name == section {
			return sec.path
		}
	}
	return ""
}

func baseURL() string {
	if b := os.Getenv("OHAX_BASE_URL"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://ohatwikeeper.com"
}

func browserURL(uuid, section string) string {
	u := baseURL() + "/" + uuid
	if p := pathOf(section); p != "" {
		u += "/" + p
	}
	return u
}

// fetch はcurl向けカードを取得する。?nocolorはohax.pw/ユーザーページではリダイレクトで
// 落ちることがあるので使わず、色なしはクライアント側でエスケープを除去して実現する。
func fetch(uuid, path string, opts options) (string, error) {
	u := baseURL() + "/" + uuid
	if path != "" {
		u += "/" + path
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ohax-cli/"+version+" (curl compatible)")
	req.Header.Set("Accept", "text/plain")
	client := &http.Client{Timeout: opts.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("取得に失敗しました: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("読み込みに失敗しました: %w", err)
	}
	body := string(b)
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		// curl向けの応答ではない(nginxのエラーページ等)。HTMLは表示しない。
		return "", fmt.Errorf("予期しない応答です (HTTP %d, %s): %s", resp.StatusCode, ct, u)
	}
	if !opts.color {
		body = stripANSI(body)
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(stripANSI(body))
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, u)
		}
		return "", errors.New(msg)
	}
	return body, nil
}

func cmdAll(uuid string, opts options) int {
	bodies := make([]string, len(sections))
	errs := make([]error, len(sections))
	var wg sync.WaitGroup
	for i, sec := range sections {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			bodies[i], errs[i] = fetch(uuid, path, opts)
		}(i, sec.path)
	}
	wg.Wait()
	code := 0
	for i := range sections {
		if i > 0 {
			fmt.Println()
		}
		if errs[i] != nil {
			fmt.Fprintf(os.Stderr, "ohax: %s: %v\n", sections[i].label, errs[i])
			code = 1
			// ユーザー不在等は全セクション共通なので1回で止める
			if i == 0 {
				return code
			}
			continue
		}
		fmt.Print(bodies[i])
	}
	return code
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]|\x1b\\]8;[^\x1b\x07]*(?:\x1b\\\\|\x07)")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// defaultColor: NO_COLORがあるか、出力が端末でなければ色なし
func defaultColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func openBrowser(u string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", u)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	return c.Start()
}

func usage(w io.Writer) {
	fmt.Fprint(w, `ohax — おはツイKeeperの公開プロフィールをターミナルで見る

使い方:
  ohax [<user>]                 プロフィール
  ohax graph   [<user>]         推移グラフ
  ohax grass   [<user>]         投稿グラス(直近12週間)
  ohax awards  [<user>]         アワード
  ohax gallery [<user>]         ギャラリー(直近20件)
  ohax rss     [<user>]         RSSフィード(XML)
  ohax all     [<user>]         プロフィール〜ギャラリーをまとめて表示

  ohax use <user>               既定ユーザーを保存(以後<user>を省略できる)
  ohax use --clear              既定ユーザーを削除
  ohax whoami                   既定ユーザーを表示
  ohax open [<section>] [<user>]  ブラウザで開く
  ohax url  [<section>] [<user>]  ブラウザ用URLを表示
  ohax version

<user> には public_uuid(例: 5axwn)か、URLをそのまま渡せる:
  https://5axwn.ohax.pw/graph, ohax.pw/5axwn, ohatwikeeper.com/5axwn/awards

オプション:
  -n, --no-color   色なしで表示(出力がパイプ/リダイレクトのときやNO_COLORがあるときは自動)
      --color      パイプ先でも色付きで出力
  -h, --help       このヘルプ
  -v, --version    バージョン

環境変数:
  OHAX_UUID        既定ユーザー(ohax useの保存値より優先)
  NO_COLOR         設定されていれば色なし
`)
}
