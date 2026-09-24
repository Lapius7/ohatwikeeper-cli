# ohax — おはツイKeeper CLI

[おはツイKeeper](https://ohatwikeeper.com/)の公開プロフィール・推移グラフ・投稿グラス・アワード・ギャラリーを、ターミナルの`ohax`コマンドで見るためのCLI。Go標準ライブラリのみ、依存パッケージなし。

`curl https://{public_uuid}.ohax.pw/graph` などで返ってくるカードを、UUIDの保存・URLの貼り付け・まとめ表示付きで使えるようにしたもの。

## インストール

Goがある場合:

```bash
go install github.com/lapius7/ohatwikeeper-cli/cmd/ohax@latest
```

Goがない場合(Linux / macOS):

```bash
curl -fsSL https://ohatwikeeper.com/cli/install.sh | bash
```

OS/CPUに合ったビルド済みバイナリを`~/.local/bin/ohax`に置く(`OHAX_INSTALL_DIR`で変更可)。Windowsは`https://ohatwikeeper.com/cli/dl/ohax-windows-amd64.exe`を直接ダウンロードする。

詳しい使い方: https://ohatwikeeper.com/cli

## 配布物のビルド

`./build.sh`で全OS/CPU向けにクロスコンパイルし、ohatwikeeper.comの`cli/dl/`(と`cli/install.sh`)に配置する。`OHAX_DIST_DIR=dist ./build.sh`なら手元に出すだけ。

## 使い方

```bash
ohax 5axwn                  # プロフィール
ohax graph 5axwn            # 推移グラフ
ohax grass 5axwn            # 投稿グラス(直近12週間)
ohax awards 5axwn           # アワード
ohax gallery 5axwn          # ギャラリー(直近20件)
ohax rss 5axwn              # RSSフィード(XML)
ohax all 5axwn              # プロフィール〜ギャラリーをまとめて表示
```

ユーザーの指定には、public_uuidのほかURLもそのまま渡せる。URLにセクションが含まれていればそれを表示する。

```bash
ohax https://5axwn.ohax.pw/graph
ohax ohatwikeeper.com/5axwn/awards
```

### 既定ユーザー

```bash
ohax use 5axwn              # 保存(存在確認してから保存する)
ohax                        # 以後はユーザー省略でOK
ohax grass
ohax whoami                 # 保存中のユーザー
ohax use --clear            # 削除
```

保存先は`$XDG_CONFIG_HOME/ohax/config.json`(macOSは`~/Library/Application Support/ohax/`、Windowsは`%AppData%\ohax\`)。環境変数`OHAX_UUID`があればそちらが優先される。

### ブラウザ

```bash
ohax open graph             # 推移グラフをブラウザで開く
ohax url awards 5axwn       # ブラウザ用URLを表示するだけ
```

### 色

出力先が端末なら色付き、パイプ・リダイレクト先や`NO_COLOR`設定時は自動で色なし(リンクのエスケープも除去)。`-n`/`--no-color`で強制的に色なし、`--color`でパイプ先でも色付き。

## 仕組み

- サーバー(ohatwikeeper.com)はUser-Agentに`curl`を含むリクエストへ、HTMLではなくANSIテキストのカードを返す。このCLIはUA `ohax-cli/<version> (curl compatible)` で`https://ohatwikeeper.com/<uuid>/<section>`を取得して表示しているだけ
- 色なしはサーバーの`?nocolor`ではなくクライアント側でエスケープシーケンスを除去して実現している(ユーザーページにクエリ文字列を付けると、現状クエリを落としたURLへ301リダイレクトされるため)
- `OHAX_BASE_URL`で接続先を差し替えられる(開発用)
