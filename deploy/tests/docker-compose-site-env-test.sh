#!/bin/sh
set -eu

# 站点地址 / 发信 / 人机验证 / 加密密钥要能只靠 .env 配：每个 compose 文件都得原样透传一次，
# 空值回落到 config.yaml 或内置默认（${KEY:-}），.env.example 里也得有这一项。
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

site_keys='
TOTP_ENCRYPTION_KEY
SERVER_FRONTEND_URL
SMTP_HOST
SMTP_PORT
SMTP_USERNAME
SMTP_PASSWORD
SMTP_FROM
SMTP_USE_TLS
TURNSTILE_SITE_KEY
TURNSTILE_SECRET_KEY
'

for key in $site_keys; do
  example_count=$(grep -Ec "^${key}=" deploy/.env.example || true)
  if [ "$example_count" -ne 1 ]; then
    printf 'deploy/.env.example must list %s exactly once\n' "$key" >&2
    exit 1
  fi
done

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  for key in $site_keys; do
    expected=$(printf '      - %s=${%s:-}' "$key" "$key")
    expected_count=$(grep -Fxc "$expected" "$compose_file" || true)
    key_count=$(grep -Ec "^[[:space:]]*-[[:space:]]*${key}([[:space:]]*=.*)?[[:space:]]*$" "$compose_file" || true)
    if [ "$expected_count" -ne 1 ] || [ "$key_count" -ne 1 ]; then
      printf '%s must pass %s with an empty fallback exactly once\n' "$compose_file" "$key" >&2
      exit 1
    fi
  done
done

printf 'docker compose site environment test passed\n'
