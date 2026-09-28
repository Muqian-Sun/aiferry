#!/usr/bin/env bash
# 本地构建 AiFerry 镜像的快速脚本，避免在命令行反复输入构建参数。
# 产出 aiferry:<版本> 和 aiferry:latest（compose 默认用 aiferry:${AIFERRY_VERSION:-latest}）。
# 版本取 backend/scripts/resolve-version.sh：精确 tag 优先，否则 backend/cmd/server/VERSION。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
VERSION="$("${REPO_ROOT}/backend/scripts/resolve-version.sh")"

docker build \
    -t "aiferry:${VERSION}" \
    -t aiferry:latest \
    --build-arg VERSION="${VERSION}" \
    --build-arg GOPROXY=https://goproxy.cn,direct \
    --build-arg GOSUMDB=sum.golang.google.cn \
    -f "${REPO_ROOT}/Dockerfile" \
    "${REPO_ROOT}"
