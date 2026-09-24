package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type config struct {
	UUID string `json:"uuid"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ohax", "config.json"), nil
}

func loadConfig() config {
	var c config
	p, err := configPath()
	if err != nil {
		return c
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}

func saveConfig(c config) (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return p, os.WriteFile(p, append(b, '\n'), 0o644)
}

// defaultUUID: OHAX_UUID → 保存済み設定
func defaultUUID() string {
	if v := os.Getenv("OHAX_UUID"); v != "" {
		if u, _, err := parseTarget(v); err == nil {
			return u
		}
	}
	return loadConfig().UUID
}

func cmdUse(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "使い方: ohax use <public_uuid または URL>  /  ohax use --clear")
		return 2
	}
	if args[0] == "--clear" {
		p, err := configPath()
		if err == nil {
			err = os.Remove(p)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "%s %v\n", paint(cRed, "✗"), err)
			return 1
		}
		fmt.Println(paint(cGreen, "✓") + " 既定ユーザーを削除しました")
		return 0
	}
	uuid, _, err := parseTarget(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", paint(cRed, "✗"), err)
		return 2
	}
	// 存在しないユーザーを保存しないよう、プロフィールが取れるか確かめてから保存する
	u, err := getUser(uuid)
	if err != nil {
		return fail(err, uuid)
	}
	p, err := saveConfig(config{UUID: uuid})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s 設定を保存できませんでした: %v\n", paint(cRed, "✗"), err)
		return 1
	}
	fmt.Printf("%s 既定ユーザーを %s %s にしました\n", paint(cGreen, "✓"), bold(u.name()), dim("("+uuid+")"))
	fmt.Println(dim("  保存先: " + p))
	fmt.Println(dim("  これからは ") + paint(cSky, "ohax profile") + dim(" や ") + paint(cSky, "ohax grass") + dim(" のようにUUIDを省略できます"))
	return 0
}

func cmdWhoami() int {
	if v := os.Getenv("OHAX_UUID"); v != "" {
		fmt.Printf("%s %s\n", bold(defaultUUID()), dim("(環境変数 OHAX_UUID)"))
		return 0
	}
	if u := loadConfig().UUID; u != "" {
		fmt.Println(u)
		return 0
	}
	fmt.Fprintln(os.Stderr, dim("既定ユーザーは未設定です(")+paint(cSky, "ohax use <public_uuid>")+dim(" で設定できます)"))
	return 1
}
