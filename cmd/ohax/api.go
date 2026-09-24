package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// おはツイKeeperの公開API(/api/v2/public/users/{uuid}/*)のクライアント

var httpClient = &http.Client{Timeout: defaultTimeout}

// errNotFound はユーザーが存在しない・非公開のとき
var errNotFound = errors.New("not found")

func apiGet(path string, v any) error {
	u := baseURL() + "/api/v2/public/" + path
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ohax-cli/"+version)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("おはツイKeeperに接続できませんでした: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("読み込みに失敗しました: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return fmt.Errorf("おはツイKeeperがエラーを返しました (HTTP %d): %s", resp.StatusCode, e.Error)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("応答を解釈できませんでした: %w", err)
	}
	return nil
}

// flexNum はPHP側で数値が文字列やnullで来ても受け取れる数値
type flexNum float64

func (f *flexNum) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexNum(v)
	return nil
}

func (f flexNum) Int() int { return int(f) }

type apiUser struct {
	PublicUUID  string          `json:"public_uuid"`
	XUsername   string          `json:"x_username"`
	DisplayName string          `json:"display_name"`
	AvatarURL   string          `json:"avatar_url"`
	Description string          `json:"description"`
	Location    string          `json:"location"`
	Website     json.RawMessage `json:"website"`
	Followers   flexNum         `json:"followers_count"`
	Following   flexNum         `json:"following_count"`
	JoinedAt    string          `json:"joined_at"`
	Total       flexNum         `json:"total_records"`
	FirstPost   string          `json:"first_post_date"`
	LastPost    string          `json:"last_post_date"`
}

func (u apiUser) name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.XUsername != "" {
		return u.XUsername
	}
	return u.PublicUUID
}

// websiteURL は website が文字列でも {url, display_url} でも受け付ける
func (u apiUser) websiteURL() (url, label string) {
	var s string
	if json.Unmarshal(u.Website, &s) == nil && s != "" {
		return s, s
	}
	var o struct {
		URL        string `json:"url"`
		DisplayURL string `json:"display_url"`
	}
	if json.Unmarshal(u.Website, &o) == nil && o.URL != "" {
		if o.DisplayURL == "" {
			o.DisplayURL = o.URL
		}
		return o.URL, o.DisplayURL
	}
	return "", ""
}

type apiStats struct {
	TotalRecords  flexNum `json:"total_records"`
	TotalLikes    flexNum `json:"total_likes"`
	TotalReposts  flexNum `json:"total_reposts"`
	TotalViews    flexNum `json:"total_views"`
	CurrentStreak flexNum `json:"current_streak"`
	MaxStreak     flexNum `json:"max_streak"`
	AvgLikes      flexNum `json:"avg_likes"`
	PostRate      flexNum `json:"post_rate"`
	FirstPost     string  `json:"first_post_date"`
	LastPost      string  `json:"last_post_date"`
}

type calDay struct {
	Count flexNum `json:"count"`
	Likes flexNum `json:"likes"`
	Views flexNum `json:"views"`
}

type apiRecord struct {
	URL           string  `json:"url"`
	Text          string  `json:"text"`
	Date          string  `json:"date"`
	Likes         flexNum `json:"likes"`
	Retweets      flexNum `json:"retweets"`
	Replies       flexNum `json:"replies"`
	Views         flexNum `json:"views"`
	ImageURL      string  `json:"image_url"`
	VideoURL      string  `json:"video_url"`
	ImageProxyURL string  `json:"image_proxy_url"`
}

func (r apiRecord) day() string {
	if len(r.Date) >= 10 {
		return r.Date[:10]
	}
	return r.Date
}

type apiRecords struct {
	Records []apiRecord `json:"records"`
	Meta    struct {
		Total flexNum `json:"total"`
	} `json:"meta"`
}

type apiAwards struct {
	Awards []struct {
		Category  string  `json:"category"`
		Threshold flexNum `json:"threshold"`
	} `json:"awards"`
	Milestones []struct {
		Category  string  `json:"category"`
		Threshold flexNum `json:"threshold"`
		Progress  flexNum `json:"progress"`
		Remaining flexNum `json:"remaining"`
	} `json:"milestones"`
	Stats struct {
		TotalPosts    flexNum `json:"total_posts"`
		TotalLikes    flexNum `json:"total_likes"`
		CurrentStreak flexNum `json:"current_streak"`
		MaxStreak     flexNum `json:"max_streak"`
	} `json:"stats"`
}

func getUser(uuid string) (apiUser, error) {
	var u apiUser
	return u, apiGet("users/"+uuid, &u)
}

func getStats(uuid string) (apiStats, error) {
	var s apiStats
	return s, apiGet("users/"+uuid+"/stats", &s)
}

func getAwards(uuid string) (apiAwards, error) {
	var a apiAwards
	return a, apiGet("users/"+uuid+"/awards", &a)
}

// getCalendar は日付(YYYY-MM-DD)→日別集計。データが無いとPHPは[]を返すので空mapにする
func getCalendar(uuid string) (map[string]calDay, error) {
	var raw struct {
		Calendar json.RawMessage `json:"calendar"`
	}
	if err := apiGet("users/"+uuid+"/calendar", &raw); err != nil {
		return nil, err
	}
	cal := map[string]calDay{}
	if len(raw.Calendar) > 0 && bytes.TrimSpace(raw.Calendar)[0] == '{' {
		if err := json.Unmarshal(raw.Calendar, &cal); err != nil {
			return nil, fmt.Errorf("応答を解釈できませんでした: %w", err)
		}
	}
	return cal, nil
}

func getRecords(uuid string, limit, offset int) (apiRecords, error) {
	var r apiRecords
	return r, apiGet(fmt.Sprintf("users/%s/records?limit=%d&offset=%d", uuid, limit, offset), &r)
}

// getAllRecords は全投稿を日付の古い順で返す(500件ずつ並列取得、上限maxRecords件)
func getAllRecords(uuid string) ([]apiRecord, error) {
	const page, maxRecords = 500, 20000
	first, err := getRecords(uuid, page, 0)
	if err != nil {
		return nil, err
	}
	total := min(first.Meta.Total.Int(), maxRecords)
	pages := [][]apiRecord{first.Records}
	if total > page {
		n := (total + page - 1) / page
		rest := make([][]apiRecord, n-1)
		errs := make([]error, n-1)
		var wg sync.WaitGroup
		for i := 1; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r, err := getRecords(uuid, page, i*page)
				rest[i-1], errs[i-1] = r.Records, err
			}(i)
		}
		wg.Wait()
		for _, e := range errs {
			if e != nil {
				return nil, e
			}
		}
		pages = append(pages, rest...)
	}
	var all []apiRecord
	for _, p := range pages {
		all = append(all, p...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Date < all[j].Date })
	return all, nil
}

// parallel は関数を並列に実行し、最初のエラーを返す
func parallel(fns ...func() error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func(i int, fn func() error) {
			defer wg.Done()
			errs[i] = fn()
		}(i, fn)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
