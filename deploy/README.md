# AiFerry Deployment Files

This directory contains the Docker Compose deployment for AiFerry. Docker Compose is
the only supported deployment method. The `aiferry` image is built locally from this
repository (repo root `Dockerfile`) and is never pulled from or pushed to a registry.

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | App + PostgreSQL + Redis (named volumes) |
| `docker-compose.local.yml` | App + PostgreSQL + Redis (local directories, easy migration) |
| `docker-compose.standalone.yml` | App only; PostgreSQL and Redis are provided externally |
| `docker-compose.dev.yml` | Local development build |
| `.env.example` | Container environment variables template |
| `config.example.yaml` | Full configuration file example (optional mount at `/app/data/config.yaml`) |
| `build_image.sh` | Builds `aiferry:<version>` and `aiferry:latest` from the repo root `Dockerfile` |
| `docker-entrypoint.sh` | Image entrypoint: fixes `/app/data` ownership, then runs as the `aiferry` user |
| `Caddyfile` | Caddy reverse proxy example |
| `EDGE_SECURITY.md` | Reverse proxy, CDN/WAF, trusted proxy, and ingress hardening guide |

---

## Docker Deployment

### Quick Start

```bash
git clone https://github.com/Muqian-Sun/aiferry.git
cd aiferry/deploy

# Configure environment
cp .env.example .env
chmod 600 .env
nano .env  # Set POSTGRES_PASSWORD and a fixed TOTP_ENCRYPTION_KEY
           # (generate with: openssl rand -hex 32); optionally the site URL,
           # SMTP and Turnstile

# Create data directories (local directory version)
mkdir -p data postgres_data redis_data

# Build the image and start all services
docker compose -f docker-compose.local.yml up -d --build

# View logs (check for auto-generated admin password)
docker compose -f docker-compose.local.yml logs -f aiferry

# User site:     http://localhost:8080
# Admin console: http://127.0.0.1:8081 (bound to ADMIN_BIND_HOST, localhost by default)
```

### Image

Every compose file (except `docker-compose.dev.yml`) uses
`image: aiferry:${AIFERRY_VERSION:-latest}` together with a `build:` section that
points at the repo root `Dockerfile`:

- `docker compose up -d --build` builds the image from the current checkout and tags it
  `aiferry:${AIFERRY_VERSION:-latest}`.
- `./build_image.sh` builds `aiferry:<version>` and `aiferry:latest`. The version comes from
  `backend/scripts/resolve-version.sh` (an exact git tag if present, otherwise
  `backend/cmd/server/VERSION`). Set `AIFERRY_VERSION=<version>` in `.env` to pin that build.

### Deployment Version Comparison

| Version | Data Storage | Migration | Best For |
|---------|-------------|-----------|----------|
| **docker-compose.local.yml** | Local directories (./data, ./postgres_data, ./redis_data) | ✅ Easy (tar entire directory) | Production, need frequent backups/migration |
| **docker-compose.yml** | Named volumes (/var/lib/docker/volumes/) | ⚠️ Requires docker commands | Simple setup, don't need migration |
| **docker-compose.standalone.yml** | Named volume for app data; external PostgreSQL / Redis | Depends on your database hosting | Managed database / Redis |

**Recommendation:** Use `docker-compose.local.yml` for easier data management and migration.

### How Auto-Setup Works

When using Docker Compose with `AUTO_SETUP=true`:

1. On first run, the system automatically:
   - Connects to PostgreSQL and Redis
   - Applies database migrations (SQL files in `backend/migrations/*.sql`) and records them in `schema_migrations`
   - Generates JWT secret (if not provided)
   - Creates admin account (password auto-generated if not provided)
   - Writes config.yaml

2. No manual Setup Wizard needed - just configure `.env` and start

   Do not place or mount a `config.yaml` before the first start: if
   `/app/data/config.yaml` already exists, auto-setup is skipped and **no admin
   account is created**. The site URL, SMTP and Turnstile can all be set in `.env`;
   use a `config.yaml` mount only for other settings, and only after the first start.

3. If `ADMIN_PASSWORD` is not set, check logs for the generated password:
   ```bash
   docker compose logs aiferry | grep "admin password"
   ```

### Startup and Database Recovery

AiFerry applies database migrations during application startup. PostgreSQL can
remain in its recovery/startup phase briefly after a host or Docker daemon
restart. The application retries transient PostgreSQL startup and connection
errors with bounded exponential backoff, then starts automatically when the
database becomes ready. Authentication errors, migration checksum mismatches,
SQL errors, and other permanent configuration or data errors fail immediately.

The Compose example also uses a PostgreSQL health check that verifies both
server readiness and a simple SQL query. `depends_on: condition: service_healthy`
controls dependency ordering for a fresh Compose start, but it is not a
replacement for application-level retries when Docker restores existing
containers after a host restart.

### Database Migration Notes (PostgreSQL)

- Migrations are applied in lexicographic order (e.g. `001_...sql`, `002_...sql`).
- `schema_migrations` tracks applied migrations (filename + checksum).
- Migrations are forward-only; rollback requires a DB backup restore or a manual compensating SQL script.

### Commands

For **local directory version** (docker-compose.local.yml):

```bash
# Start services (builds the image if needed)
docker compose -f docker-compose.local.yml up -d --build

# Stop services
docker compose -f docker-compose.local.yml down

# View logs
docker compose -f docker-compose.local.yml logs -f aiferry

# Restart AiFerry only
docker compose -f docker-compose.local.yml restart aiferry

# Upgrade: update the checkout, rebuild the image, recreate the container
git pull
docker compose -f docker-compose.local.yml up -d --build

# Remove all data (caution!)
docker compose -f docker-compose.local.yml down
rm -rf data/ postgres_data/ redis_data/
```

For **named volumes version** (docker-compose.yml):

```bash
# Start services (builds the image if needed)
docker compose up -d --build

# Stop services
docker compose down

# View logs
docker compose logs -f aiferry

# Restart AiFerry only
docker compose restart aiferry

# Upgrade: update the checkout, rebuild the image, recreate the container
git pull
docker compose up -d --build

# Remove all data (caution!)
docker compose down -v
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTGRES_PASSWORD` | **Yes** | - | PostgreSQL password |
| `TOTP_ENCRYPTION_KEY` | **Yes** | *(random per start)* | Encrypts stored secrets (2FA, channel monitor keys, ...); 2FA is only offered when it is set. An empty value gets a new random key on every start, so data encrypted before a restart can no longer be read. Set it once (`openssl rand -hex 32`) and never change it |
| `JWT_SECRET` | No | *(auto-generated)* | Login token signing key; generated on first start and stored in the database, so sessions survive restarts without it |
| `BIND_HOST` | No | `0.0.0.0` | Host address the user site port binds to; use `127.0.0.1` behind a reverse proxy on the same host |
| `SERVER_FRONTEND_URL` | No | - | Public user site URL (e.g. `https://example.com`), used for links in emails and third-party login callbacks |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` / `SMTP_USE_TLS` | No | port `587`, TLS `false` | Outgoing mail. Email verification and alert emails turn on when `SMTP_HOST` and `SMTP_FROM` are set; password reset also needs `SERVER_FRONTEND_URL`. `SMTP_USE_TLS=true` = implicit TLS (port 465); `false` = STARTTLS (port 587) |
| `TURNSTILE_SITE_KEY` / `TURNSTILE_SECRET_KEY` | No | - | Cloudflare Turnstile captcha; on when both are set. Setting only one of them fails startup |
| `AIFERRY_VERSION` | No | `latest` | Tag of the locally built `aiferry` image used by compose |
| `SERVER_PORT` | No | `8080` | User site port on the host |
| `SERVER_ADMIN_PORT` | No | `8081` | Admin console port on the host |
| `ADMIN_BIND_HOST` | No | `127.0.0.1` | Host address the admin console port binds to |
| `ADMIN_EMAIL` | No | `admin@aiferry.local` | Admin email |
| `ADMIN_PASSWORD` | No | *(auto-generated)* | Admin password |
| `TZ` | No | `Asia/Shanghai` | Timezone |
| `GEMINI_OAUTH_CLIENT_ID` | No | *(builtin)* | Google OAuth client ID (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_CLIENT_SECRET` | No | *(builtin)* | Google OAuth client secret (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_SCOPES` | No | *(default)* | OAuth scopes (Gemini OAuth) |
| `GEMINI_QUOTA_POLICY` | No | *(empty)* | JSON overrides for Gemini local quota simulation (Code Assist only). |

See `.env.example` for all available options.

### Easy Migration (Local Directory Version)

When using `docker-compose.local.yml`, all data is stored in local directories, making migration simple:

```bash
# On source server: Stop services and create archive
cd /path/to/aiferry/deploy
docker compose -f docker-compose.local.yml down
cd ..
tar czf aiferry-complete.tar.gz deploy/

# Transfer to new server
scp aiferry-complete.tar.gz user@new-server:/path/to/destination/

# On new server: clone the repository (needed to build the image),
# extract the archive over its deploy/ directory, then start
git clone https://github.com/Muqian-Sun/aiferry.git
cd aiferry
tar xzf /path/to/destination/aiferry-complete.tar.gz
cd deploy
docker compose -f docker-compose.local.yml up -d --build
```

Your entire deployment (configuration + data) is migrated!

### Reverse Proxy Notes

- A Caddy example lives in `Caddyfile`; see [EDGE_SECURITY.md](./EDGE_SECURITY.md) for CDN/WAF,
  trusted proxy, and ingress hardening.
- With the proxy on the same host, set `BIND_HOST=127.0.0.1` so the user site is only reachable
  through the proxy. Under Docker, AiFerry sees the Docker bridge gateway (e.g. `172.19.0.1`) as
  the peer, not `127.0.0.1`: leave `server.trusted_proxies` unset, or include the Docker range
  (see EDGE_SECURITY.md). Listing only `127.0.0.1` records every client under the gateway address.
- When using Nginx in front of AiFerry with Codex CLI, add `underscores_in_headers on;` to the
  `http` block. Nginx drops headers containing underscores by default (e.g. `session_id`),
  which breaks sticky session routing in multi-account setups.

---

## Gemini OAuth Configuration

AiFerry supports three methods to connect to Gemini:

### Method 1: Code Assist OAuth (Recommended for GCP Users)

**No configuration needed** - always uses the built-in Gemini CLI OAuth client (public).

1. Leave `GEMINI_OAUTH_CLIENT_ID` and `GEMINI_OAUTH_CLIENT_SECRET` empty
2. In the Admin UI, create a Gemini OAuth account and select **"Code Assist"** type
3. Complete the OAuth flow in your browser

> Note: Even if you configure `GEMINI_OAUTH_CLIENT_ID` / `GEMINI_OAUTH_CLIENT_SECRET` for AI Studio OAuth,
> Code Assist OAuth will still use the built-in Gemini CLI client.

**Requirements:**
- Google account with access to Google Cloud Platform
- A GCP project (auto-detected or manually specified)

**How to get Project ID (if auto-detection fails):**
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page
3. Copy the Project ID (not the project name) from the list
4. Common formats: `my-project-123456` or `cloud-ai-companion-xxxxx`

### Method 2: AI Studio OAuth (For Regular Google Accounts)

Requires your own OAuth client credentials.

**Step 1: Create OAuth Client in Google Cloud Console**

1. Go to [Google Cloud Console - Credentials](https://console.cloud.google.com/apis/credentials)
2. Create a new project or select an existing one
3. **Enable the Generative Language API:**
   - Go to "APIs & Services" → "Library"
   - Search for "Generative Language API"
   - Click "Enable"
4. **Configure OAuth Consent Screen** (if not done):
   - Go to "APIs & Services" → "OAuth consent screen"
   - Choose "External" user type
   - Fill in app name, user support email, developer contact
   - Add scopes: `https://www.googleapis.com/auth/generative-language.retriever` (and optionally `https://www.googleapis.com/auth/cloud-platform`)
   - Add test users (your Google account email)
5. **Create OAuth 2.0 credentials:**
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   - Application type: **Web application** (or **Desktop app**)
   - Name: e.g., "AiFerry Gemini"
   - Authorized redirect URIs: Add `http://localhost:1455/auth/callback`
6. Copy the **Client ID** and **Client Secret**
7. **⚠️ Publish to Production (IMPORTANT):**
   - Go to "APIs & Services" → "OAuth consent screen"
   - Click "PUBLISH APP" to move from Testing to Production
   - **Testing mode limitations:**
     - Only manually added test users can authenticate (max 100 users)
     - Refresh tokens expire after 7 days
     - Users must be re-added periodically
   - **Production mode:** Any Google user can authenticate, tokens don't expire
   - Note: For sensitive scopes, Google may require verification (demo video, privacy policy)

**Step 2: Configure Environment Variables**

```bash
GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret

# 可选：如需使用 Gemini CLI 内置 OAuth Client（Code Assist / Google One）
# 安全说明：本仓库不会内置该 client_secret，请在运行环境通过环境变量注入。
# GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
```

**Step 3: Create Account in Admin UI**

1. Create a Gemini OAuth account and select **"AI Studio"** type
2. Complete the OAuth flow
   - After consent, your browser will be redirected to `http://localhost:1455/auth/callback?code=...&state=...`
   - Copy the full callback URL (recommended) or just the `code` and paste it back into the Admin UI

### Method 3: API Key (Simplest)

1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Click "Create API key"
3. In Admin UI, create a Gemini **API Key** account
4. Paste your API key (starts with `AIza...`)

### Comparison Table

| Feature | Code Assist OAuth | AI Studio OAuth | API Key |
|---------|-------------------|-----------------|---------|
| Setup Complexity | Easy (no config) | Medium (OAuth client) | Easy |
| GCP Project Required | Yes | No | No |
| Custom OAuth Client | No (built-in) | Yes (required) | N/A |
| Rate Limits | GCP quota | Standard | Standard |
| Best For | GCP developers | Regular users needing OAuth | Quick testing |

---

## Troubleshooting

For **local directory version**:

```bash
# Check container status
docker compose -f docker-compose.local.yml ps

# View detailed logs
docker compose -f docker-compose.local.yml logs --tail=100 aiferry

# Check database connection
docker compose -f docker-compose.local.yml exec postgres pg_isready

# Check Redis connection
docker compose -f docker-compose.local.yml exec redis redis-cli ping

# Restart all services
docker compose -f docker-compose.local.yml restart

# Check data directories
ls -la data/ postgres_data/ redis_data/
```

For **named volumes version**:

```bash
# Check container status
docker compose ps

# View detailed logs
docker compose logs --tail=100 aiferry

# Check database connection
docker compose exec postgres pg_isready

# Check Redis connection
docker compose exec redis redis-cli ping

# Restart all services
docker compose restart
```

### Common Issues

1. **Port already in use**: Change `SERVER_PORT` / `SERVER_ADMIN_PORT` in `.env`
2. **Database connection failed**: Check PostgreSQL is running and credentials are correct
3. **Redis connection failed**: Check Redis is running and password is correct
4. **Image not found**: Run `docker compose up -d --build` (or `./build_image.sh`) to build `aiferry:${AIFERRY_VERSION:-latest}` locally

---

## TLS Fingerprint Configuration

AiFerry supports TLS fingerprint simulation to make requests appear as if they come from the official Claude CLI (Node.js client).

### Default Behavior

- Built-in `claude_cli_v2` profile simulates Node.js 20.x + OpenSSL 3.x
- JA3 Hash: `1a28e69016765d92e3b381168d68922c`
- JA4: `t13d5911h1_a33745022dd6_1f22a2ca17c4`
- Profile selection: `accountID % profileCount`

### Configuration

```yaml
gateway:
  tls_fingerprint:
    enabled: true  # Global switch
    profiles:
      # Simple profile (uses default cipher suites)
      profile_1:
        name: "Profile 1"

      # Profile with custom cipher suites (use compact array format)
      profile_2:
        name: "Profile 2"
        cipher_suites: [4866, 4867, 4865, 49199, 49195, 49200, 49196]
        curves: [29, 23, 24]
        point_formats: 0

      # Another custom profile
      profile_3:
        name: "Profile 3"
        cipher_suites: [4865, 4866, 4867, 49199, 49200]
        curves: [29, 23, 24, 25]
```

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name (required) |
| `cipher_suites` | []uint16 | Cipher suites in decimal. Empty = default |
| `curves` | []uint16 | Elliptic curves in decimal. Empty = default |
| `point_formats` | []uint8 | EC point formats. Empty = default |

### Common Values Reference

**Cipher Suites (TLS 1.3):** `4865` (AES_128_GCM), `4866` (AES_256_GCM), `4867` (CHACHA20)

**Cipher Suites (TLS 1.2):** `49195`, `49196`, `49199`, `49200` (ECDHE variants)

**Curves:** `29` (X25519), `23` (P-256), `24` (P-384), `25` (P-521)
