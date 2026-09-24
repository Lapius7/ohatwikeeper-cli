#!/usr/bin/env bash
# ohax (おはツイKeeper 公式CLI) のワンライナーインストーラー。
#
#   curl -fsSL https://ohatwikeeper.com/cli/install.sh | bash
#
# Goは不要。ohatwikeeper.com/cli/dl/ からOS・CPUに合ったビルド済みバイナリを取得して
# ~/.local/bin/ohax に置く(OHAX_INSTALL_DIRで変更可)。
set -euo pipefail

BASE_URL="${OHAX_DL_URL:-https://ohatwikeeper.com/cli/dl}"
INSTALL_DIR="${OHAX_INSTALL_DIR:-$HOME/.local/bin}"

if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RESET=$'\033[0m'
  RED=$'\033[31m'; GREEN=$'\033[32m'; CYAN=$'\033[36m'; YELLOW=$'\033[33m'
else
  BOLD=""; DIM=""; RESET=""; RED=""; GREEN=""; CYAN=""; YELLOW=""
fi
info() { printf "%s→%s %s\n" "$CYAN" "$RESET" "$1"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"; }
err()  { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; }

printf "%sohax%s — おはツイKeeper 公式CLI インストーラー\n\n" "$BOLD" "$RESET"

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) err "未対応のOSです: $(uname -s)(Windowsは ${BASE_URL}/ohax-windows-amd64.exe を直接ダウンロードしてください)"; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) err "未対応のCPUです: $(uname -m)"; exit 1 ;;
esac

VERSION="$(curl -fsSL "${BASE_URL}/VERSION" 2>/dev/null || echo "?")"
info "ohax ${VERSION} (${OS}/${ARCH}) をダウンロード中"

mkdir -p "$INSTALL_DIR"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
if ! curl -fsSL "${BASE_URL}/ohax-${OS}-${ARCH}" -o "$TMP"; then
  err "ダウンロードに失敗しました"
  exit 1
fi
# 配布元のSHA256SUMSと照合する(取得できなければ省略)
if SUMS="$(curl -fsSL "${BASE_URL}/SHA256SUMS" 2>/dev/null)"; then
  WANT="$(printf '%s\n' "$SUMS" | awk -v f="ohax-${OS}-${ARCH}" '$2==f{print $1}')"
  if command -v sha256sum >/dev/null; then GOT="$(sha256sum "$TMP" | cut -d' ' -f1)"; else GOT="$(shasum -a 256 "$TMP" | cut -d' ' -f1)"; fi
  if [ -n "$WANT" ] && [ "$WANT" != "$GOT" ]; then
    err "チェックサムが一致しません。もう一度実行してください"
    exit 1
  fi
fi
chmod +x "$TMP"
mv "$TMP" "${INSTALL_DIR}/ohax"
trap - EXIT
ok "インストール先: ${DIM}${INSTALL_DIR}/ohax${RESET}"
ok "バージョン: ${BOLD}$("${INSTALL_DIR}/ohax" version 2>/dev/null | sed 's/^ohax //')${RESET}"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf "\n"
    warn "PATHに ${INSTALL_DIR} が通っていません"
    printf "  シェルの設定ファイル(~/.bashrc, ~/.zshrc 等)に以下を追記してください:\n"
    printf "  %sexport PATH=\"\$PATH:%s\"%s\n" "$DIM" "$INSTALL_DIR" "$RESET"
    ;;
esac

printf "\n%s🎉 ohax のインストールが完了しました！%s\n\n" "$BOLD" "$RESET"
printf "次のステップ:\n"
printf "  %s1.%s %sohax use <public_uuid>%s  既定ユーザーを保存\n" "$BOLD" "$RESET" "$CYAN" "$RESET"
printf "  %s2.%s %sohax all%s                プロフィール〜ギャラリーをまとめて表示\n" "$BOLD" "$RESET" "$CYAN" "$RESET"
printf "\n%s詳細:%s https://ohatwikeeper.com/cli\n" "$DIM" "$RESET"
