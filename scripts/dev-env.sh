#!/usr/bin/env bash
# 开发环境变量：把项目自带的 Go 工具链放到 PATH 最前。
# 用法：source scripts/dev-env.sh

export NOVAMIND_GO_ROOT="${HOME}/.local/go"
if [ -x "${NOVAMIND_GO_ROOT}/bin/go" ]; then
  export PATH="${NOVAMIND_GO_ROOT}/bin:${PATH}"
fi

# 本地开发默认值（可被 .env 覆盖）
export NOVAMIND_ENV="${NOVAMIND_ENV:-development}"

echo "go:    $(command -v go) ($(go version 2>/dev/null | awk '{print $3}'))"
echo "node:  $(command -v node) ($(node -v 2>/dev/null))"
