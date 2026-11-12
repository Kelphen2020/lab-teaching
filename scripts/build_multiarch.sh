#!/bin/bash
# V1.0 国产化多架构构建脚本
# 构建后端(Go: amd64/arm64/loong64) + Agent(Rust: amd64/aarch64/loongarch64)
# 目标环境：统信UOS / 银河麒麟 / 中标麒麟 + 海光/兆芯/飞腾/鲲鹏/龙芯
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

source ~/.cargo/env 2>/dev/null || true
export PATH="$HOME/zig:$HOME/bin:$PATH"

echo "=== [1/2] 构建后端 Gateway (Go, CGO_ENABLED=0 静态) ==="
cd "$ROOT/backend"
for arch in amd64 arm64 loong64; do
    echo "  -> linux/$arch"
    GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=$arch \
        go build -ldflags="-s -w" -o "$OUT/gateway-api-linux-$arch" ./cmd/gateway-api
done

echo "=== [2/2] 构建 Agent (Rust) ==="
cd "$ROOT/agent"

echo "  -> x86_64 (海光/兆芯, 统信UOS/麒麟)"
cargo build --release --quiet
cp target/release/lab-agent "$OUT/lab-agent-linux-amd64"

echo "  -> aarch64 (飞腾/鲲鹏, 统信UOS/麒麟)"
cargo build --release --target aarch64-unknown-linux-gnu --quiet
cp target/aarch64-unknown-linux-gnu/release/lab-agent "$OUT/lab-agent-linux-arm64"

echo "  -> loongarch64 (龙芯3A5000/3A6000, 无TLS内网版)"
cargo build --release --no-default-features --target loongarch64-unknown-linux-gnu --quiet \
    --config 'target.loongarch64-unknown-linux-gnu.linker="'$HOME'/bin/zigcc-loong64"'
cp target/loongarch64-unknown-linux-gnu/release/lab-agent "$OUT/lab-agent-linux-loong64"

echo ""
echo "=== 构建产物 ==="
ls -lh "$OUT"
echo ""
echo "说明："
echo "  gateway-api-linux-amd64  : x86_64 服务器（通用）"
echo "  gateway-api-linux-arm64  : 鲲鹏920/飞腾 服务器"
echo "  gateway-api-linux-loong64: 龙芯 3A5000/3A6000 服务器"
echo "  lab-agent-linux-amd64    : 海光/兆芯 学生机（统信UOS/麒麟 x86）"
echo "  lab-agent-linux-arm64    : 飞腾/鲲鹏 学生机（统信UOS/麒麟 ARM）"
echo "  lab-agent-linux-loong64  : 龙芯 学生机（纯内网版,无TLS）"
