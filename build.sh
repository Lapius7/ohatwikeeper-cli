#!/usr/bin/env bash
# 全OS/CPU向けにクロスコンパイルし、ohatwikeeper.comの配布ディレクトリ(cli/dl/)に置く。
# nginxが https://ohatwikeeper.com/cli/dl/ としてそのまま配信するので、再起動は不要。
# install.shは1つ上の階層(cli/install.sh)にコピーする。
#
#   ./build.sh                     # バージョンは日付+gitの短縮ハッシュ(未コミットの変更があれば末尾に-dirty)
#   VERSION=v0.1.0 ./build.sh      # タグと揃えるとき
#   OHAX_DIST_DIR=dist ./build.sh  # 手元に出すだけ
set -euo pipefail

cd "$(dirname "$0")"
OUT="${OHAX_DIST_DIR:-/root/project/git/github-privaterepo/ohatwikeeper.com/cli/dl}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
DIRTY=""
git diff --quiet HEAD 2>/dev/null || DIRTY="-dirty"
VERSION="${VERSION:-$(date +%Y.%m.%d)-${COMMIT}${DIRTY}}"

mkdir -p "$OUT"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os="${target%/*}"; arch="${target#*/}"
  ext=""; [ "$os" = windows ] && ext=".exe"
  echo "→ ${os}/${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${OUT}/ohax-${os}-${arch}${ext}" ./cmd/ohax
done
(cd "$OUT" && sha256sum ohax-* > SHA256SUMS)
printf "%s\n" "$VERSION" > "${OUT}/VERSION"
cp install.sh "${OUT}/../install.sh"
echo "✓ ${VERSION} を ${OUT} に配置しました"
