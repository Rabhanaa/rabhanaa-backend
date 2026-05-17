# Deployment Guide — Rabhana API on Coolify

This guide describes how the Rabhana API is deployed on the production VM via
**Coolify**, and how the **MinIO** object storage that the API depends on is
laid out on the same host. It is written against the live setup at
`194.163.180.176` and should be kept in sync with that server.

---

## 1. Server overview

| Item | Value |
|------|-------|
| Host | `194.163.180.176` (Ubuntu 24.04 LTS, `vmi3152148`) |
| SSH user | `admin` (sudo enabled) |
| Orchestrator | Coolify `v4.0.0-beta.469` |
| Edge proxy | `lucaslorentz/caddy-docker-proxy` (`coolify-proxy`, ports 80/443) |
| Docker network used by apps | `coolify` |
| Public domains | `rabhanaa.com` (web), `api.rabhanaa.com` (API), `storage.rabhanaa.com` (MinIO S3) |
| Coolify UI | `http://194.163.180.176:8000` |

### Directory layout on the host

```
/data/coolify/
├── applications/   # per-app workspaces; secrets like firebase.json live under <app>/secrets/
├── databases/      # managed databases (Postgres, Redis, ...)
├── backups/        # Coolify-managed backups
├── source/         # cached git checkouts for Git Source builds
├── proxy/          # caddy-docker-proxy state (autosave Caddyfile)
├── ssh/            # Coolify deploy keys
└── ssl/            # certificates
```

You almost never edit these directly — Coolify manages them. The two paths worth
knowing are `applications/<uuid>/secrets/` (where you drop secret files that get
bind-mounted into a container) and `source/` (the git cache).

### Currently running containers

```
NAME                                      ROLE
coolify                                   Coolify control plane (port 8000 -> 8080)
coolify-db (postgres:15-alpine)           Coolify's own DB
coolify-redis (redis:7-alpine)            Coolify queue/cache
coolify-proxy (caddy-docker-proxy)        Edge HTTPS reverse proxy (80/443)
coolify-sentinel                          Metrics agent
c127wknohfrrnco2sje5wmm8 (postgres:18)    Rabhana app database
c127wknohfrrnco2sje5wmm8-proxy (nginx)    Exposes Postgres on 5432 publicly
minio (minio/minio)                       Object storage (S3 API on :9000)
nihxoxm8rqp39ssui0qk7e2e-...              Rabhana API (Go) — built from this repo
r369d4w9t7i2ko3px755omoe-...              rabhanaa.com web container (port 80)
```

---

## 2. Deploying via Coolify — Git Source

The Rabhana API is deployed as a Coolify **Application** with a **Git Source**.
This section is the playbook for connecting a Git repository to Coolify and
shipping it.

### 2.1 Add the Git Source (one-time, per provider)

A "Source" is the credential Coolify uses to clone the repo and (optionally)
register webhooks.

1. Sign in to Coolify at `http://194.163.180.176:8000`.
2. Sidebar → **Sources** → **+ New Source**.
3. Pick your provider:
   - **GitHub App** (recommended) — install the Coolify GitHub App on the
     target org/repo. This gives Coolify clone access *and* registers the push
     webhook automatically. Best option for private repos.
   - **Public Repository** — no source needed; you can paste a public clone
     URL straight into the application.
   - **Generic Git (Deploy Key)** — Coolify generates an SSH key; you paste the
     public half into the repo's Deploy Keys (GitHub: *Settings → Deploy Keys
     → Add deploy key*, allow read-only access).
4. Save. The source is now reusable across applications.

> If you don't see "Sources" in the sidebar, you're inside a project — click the
> Coolify logo (top-left) to return to the global navigation.

### 2.2 Create the Application

1. Open the project (e.g. **rabhana-dev-api**) → **+ New Resource** →
   **Application** → **Public Repository** *or* **Private Repository (with
   GitHub App)**, depending on what you set up.
2. Fill in:
   - **Repository**: `https://github.com/azsharkawy5/rabhana` (the existing
     production app uses this repo — see container label
     `org.opencontainers.image.source`).
   - **Branch**: `main` (or whichever branch should auto-deploy).
   - **Build Pack**: **Dockerfile** — this repo ships its own multi-stage
     `Dockerfile` at the repo root that produces a ~30 MB Alpine image.
   - **Base Directory**: `/` (leave default).
   - **Dockerfile Location**: `/Dockerfile` (default).
   - **Port (exposed)**: `8080` — must match `EXPOSE 8080` in the Dockerfile
     and `SERVER_PORT` in the env.
3. Click **Continue**.

### 2.3 Configure the domain (HTTPS)

On the application's **General** tab:

- **Domains**: `https://api.rabhanaa.com`.
- Coolify writes Caddy labels onto the container; `coolify-proxy` picks them
  up and provisions a Let's Encrypt cert automatically. The live cert resolver
  is wired through Traefik labels too (`certresolver=letsencrypt`).
- DNS prerequisite: an A record for `api.rabhanaa.com` pointing at
  `194.163.180.176`. Same for `rabhanaa.com` and `storage.rabhanaa.com`.

After save, click **Deploy** → wait for "Running (healthy)". You can verify
externally with:

```bash
curl -I https://api.rabhanaa.com/health
```

### 2.4 Environment variables

Set these under **Environment Variables** for the application. The values
below are taken from the currently-running container — replace anything
marked **change me** before deploying to a new environment.

| Variable | Example value | Notes |
|----------|---------------|-------|
| `PORT` | `8080` | Must match the Dockerfile `EXPOSE`. |
| `HOST` | `0.0.0.0` | Bind on all interfaces inside the container. |
| `DATABASE_URL` | `postgres://postgres:<password>@c127wknohfrrnco2sje5wmm8:5432/postgres?sslmode=require` | Use the **container name** of the managed Postgres as the host — it's reachable on the `coolify` Docker network. **change me** for new envs. |
| `APP_BASE_URL` | `https://rabhanaa.com` | Public web URL. |
| `MINIO_ENDPOINT` | `storage.rabhanaa.com` | Public S3 endpoint hostname. |
| `MINIO_USE_SSL` | `true` | TLS terminated by `coolify-proxy`. |
| `MINIO_ACCESS_KEY` | `minioadmin` | **change me** — see §3.4. |
| `MINIO_SECRET_KEY` | `minioadmin` | **change me** — see §3.4. |
| `MINIO_PUBLIC_BUCKET` | `auction-images` | Bucket served openly via the proxy. |
| `MINIO_PUBLIC_BASE_URL` | `https://storage.rabhanaa.com` | Used to build the URLs returned to clients. |
| `MINIO_PRIVATE_BUCKET` | `documents` | Pre-signed-URL access only. |
| `FIREBASE_CREDENTIALS_PATH` | `/secrets/firebase.json` | See §2.5. |
| `JWT_SECRET` | *(random)* | **change me** — generate with `openssl rand -base64 48`. |
| `SEED_USER_ID` | `2001` | Optional seed admin. |

Coolify also injects its own variables automatically (`COOLIFY_URL`,
`COOLIFY_FQDN`, `COOLIFY_CONTAINER_NAME`, `SOURCE_COMMIT`, ...). You don't set
those by hand.

> Mark every secret (DB password, JWT secret, MinIO secret key, etc.) with the
> **"Is Build Variable?" = off** and **"Is Secret?" = on** toggles. Secrets are
> encrypted at rest and redacted from logs.

### 2.5 Secret files (firebase.json)

Some secrets are easier to mount as files than as env vars. Coolify supports
this via **Storages**:

1. Application → **Storages** → **Add**.
2. Type: **File Mount**.
3. **Source Path** (on host): `/data/coolify/applications/<UUID>/secrets/firebase.json`
4. **Mount Path** (in container): `/secrets/firebase.json`
5. Save, then upload the file:
   ```bash
   sshpass -p '<vm-password>' scp ./firebase.json admin@194.163.180.176:/tmp/firebase.json
   sshpass -p '<vm-password>' ssh admin@194.163.180.176 \
     "sudo install -o 9999 -g root -m 600 /tmp/firebase.json /data/coolify/applications/<UUID>/secrets/firebase.json && sudo rm /tmp/firebase.json"
   ```
   Replace `<UUID>` with the application's resource UUID (visible in the
   Coolify URL bar, and exported as `COOLIFY_RESOURCE_UUID` inside the
   container — e.g. `nihxoxm8rqp39ssui0qk7e2e` for the live one).
6. Redeploy so the bind mount is picked up.

### 2.6 Build & deploy

- **Manual deploy**: app page → **Deploy**. Coolify pulls the latest commit on
  the configured branch, runs `docker build` against the repo's `Dockerfile`,
  tags the image, and starts a new container with zero-downtime swap.
- **Auto deploy**: app → **Webhooks** → toggle **Auto Deploy on Push**. If you
  used the GitHub App, the webhook was registered for you; otherwise copy the
  shown URL and add it manually under *GitHub → repo → Settings → Webhooks*
  with content type `application/json`.
- **Preview deploys** for PRs are off by default; turn on under **Preview
  Deployments** if you want them.
- **Rollback**: app → **Deployments** → pick a previous successful build →
  **Redeploy**. Coolify keeps the image, so rollback is fast.

### 2.7 Logs, exec, health

- **Logs**: app → **Logs** (streams `docker logs` for the running container).
- **Terminal**: app → **Terminal** for an in-container shell — the live image
  runs as `nonroot:nonroot`, so apk/install isn't possible; use this for
  `cat /tmp/...`, env inspection, curl-ing `localhost:8080/health`, etc.
- **Healthcheck**: defined in the Dockerfile —
  `wget -qO- http://localhost:8080/health || exit 1`. The container shows
  `(healthy)` in `docker ps` once it passes.

### 2.8 Reproducing this without the UI (for reference)

If you ever need to know what Coolify is doing under the hood: it generates a
`docker-compose.yaml` under `/artifacts/<deploy-id>/`, applies Caddy/Traefik
labels for the proxy, then `docker compose up`s it on the `coolify` network.
See the live container's labels for the exact label set
(`coolify.applicationId`, `caddy_0.*`, `traefik.http.routers.*`).

---

## 3. MinIO storage on the VM

MinIO provides S3-compatible object storage for the API: bid/auction images,
user documents, etc. It runs as a plain Docker container alongside Coolify
(not managed by Coolify) and is reverse-proxied by `coolify-proxy` so the API
can use a real HTTPS URL.

### 3.1 Container & volume

| Field | Value |
|-------|-------|
| Container name | `minio` |
| Image | `minio/minio` |
| Command | `server /data` |
| Docker network | `coolify` |
| Host port published | `9000` (S3 API). The Web Console is **not** exposed. |
| Mount | volume `minio-data` → `/data` |
| On-disk path | `/var/lib/docker/volumes/minio-data/_data` |
| Public DNS | `storage.rabhanaa.com` → routes through `coolify-proxy` → `minio:9000` |
| Root credentials | `MINIO_ROOT_USER=minioadmin` / `MINIO_ROOT_PASSWORD=minioadmin` (**default — change in production, see §3.4**) |

The Caddy entry that publishes it (`/data/coolify/proxy/dynamic/...` /
`coolify-proxy`'s autosave Caddyfile) looks like:

```caddyfile
storage.rabhanaa.com {
    reverse_proxy 10.0.1.7:9000
}
```

> `10.0.1.7` is MinIO's IP on the `coolify` Docker network — Caddy can also
> reach it by container name (`minio:9000`); both work.

### 3.2 Buckets

Two buckets exist on disk today (`ls /var/lib/docker/volumes/minio-data/_data`):

| Bucket | Visibility | Purpose |
|--------|-----------|---------|
| `auction-images` | public-read | Auction/listing images. URLs returned to clients as `https://storage.rabhanaa.com/auction-images/<key>`. |
| `documents` | private | KYC / contract files; the API hands clients short-lived pre-signed URLs. |

The application code reads `MINIO_PUBLIC_BUCKET` and `MINIO_PRIVATE_BUCKET` to
decide where to put each upload (see `upload/` package).

### 3.3 How the API talks to MinIO

- The Go service uses the MinIO Go SDK (S3 v4 signatures).
- It connects to **`MINIO_ENDPOINT` over HTTPS** (`storage.rabhanaa.com:443`),
  not to the internal `minio:9000`. This means uploads from the API and the
  URLs handed to mobile clients both go through Caddy, so any future swap to a
  managed S3 only needs DNS + credentials changed.
- Public-bucket downloads are direct: `https://storage.rabhanaa.com/auction-images/<key>`.
- Private-bucket downloads are via pre-signed URLs the API mints on demand.

### 3.4 Initial hardening (do this before production traffic)

The MinIO container currently starts with **default credentials**
(`minioadmin:minioadmin`) — the container logs warn about this on every boot.
Rotate before going live:

```bash
# On the VM
sudo docker stop minio
sudo docker rm minio
sudo docker run -d --name minio --restart unless-stopped \
  --network coolify \
  -p 9000:9000 \
  -e MINIO_ROOT_USER='<new-admin>' \
  -e MINIO_ROOT_PASSWORD='<long-random>' \
  -v minio-data:/data \
  minio/minio server /data
```

Then update `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` in the Coolify app env and
redeploy. Even better: create a dedicated access key per app via `mc admin user
add` and grant only the buckets it needs (see §3.6) — never give the app the
root key.

### 3.5 Managing buckets with the `mc` client

The fastest way to manage MinIO from the VM is the official `mc` admin client.
You can run it as a one-shot Docker container without installing anything:

```bash
# Alias the local MinIO instance (run on the VM)
sudo docker run --rm --network coolify minio/mc \
  alias set local http://minio:9000 minioadmin minioadmin

# List buckets
sudo docker run --rm --network coolify minio/mc alias set local http://minio:9000 minioadmin minioadmin \
  && sudo docker run --rm --network coolify minio/mc ls local

# Make a bucket and set anonymous read on it
sudo docker run --rm --network coolify minio/mc mb local/new-bucket
sudo docker run --rm --network coolify minio/mc anonymous set download local/new-bucket
```

For repeated use, save the alias to a persistent config dir:

```bash
sudo docker run --rm -v ~/.mc:/root/.mc --network coolify minio/mc \
  alias set local http://minio:9000 minioadmin minioadmin
# Subsequent commands reuse ~/.mc
sudo docker run --rm -v ~/.mc:/root/.mc --network coolify minio/mc ls local
```

### 3.6 Per-app access keys

Don't ship the root credentials in your app env. Create a scoped user:

```bash
MC="sudo docker run --rm -v ~/.mc:/root/.mc --network coolify minio/mc"

$MC admin user add local rabhana-api '<long-random-secret>'

# Give it read+write on the two buckets the app uses
cat > /tmp/rabhana-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow",
      "Action": ["s3:*"],
      "Resource": ["arn:aws:s3:::auction-images", "arn:aws:s3:::auction-images/*",
                   "arn:aws:s3:::documents",      "arn:aws:s3:::documents/*"] }
  ]
}
EOF

$MC admin policy create local rabhana-app /tmp/rabhana-policy.json
$MC admin policy attach local rabhana-app --user rabhana-api
```

Then update `MINIO_ACCESS_KEY=rabhana-api` and `MINIO_SECRET_KEY=<...>` in
Coolify and redeploy.

### 3.7 Backups

The MinIO volume is **not** included in Coolify's database backup schedule —
back it up separately.

Quick nightly snapshot to another disk or off-host:

```bash
# Mirror to an external S3 / restic / borg target, e.g. mirror to a sibling bucket on a backup MinIO:
$MC mirror --overwrite --remove local/auction-images backup/auction-images
$MC mirror --overwrite --remove local/documents      backup/documents

# Or take a cold tarball (stop MinIO first to get a consistent snapshot)
sudo docker stop minio
sudo tar -C /var/lib/docker/volumes/minio-data -czf /backups/minio-$(date +%F).tgz _data
sudo docker start minio
```

Restore is the inverse: stop the container, replace `_data/`, start.

### 3.8 Console / GUI access

The MinIO Console isn't published publicly. Two ways to get into it:

1. **SSH tunnel** (no server changes):
   ```bash
   ssh -L 9001:minio:9001 admin@194.163.180.176
   ```
   …won't work directly because the MinIO console port isn't started. Add
   `--console-address ":9001"` to the container command and re-publish that
   port if you want a UI. With the current setup, prefer `mc`.

2. **Expose via Coolify**: add a new domain in Caddy (e.g.
   `console.storage.rabhanaa.com`) routing to `minio:9001`, then start the
   container with `--console-address ":9001"` and publish `9001`. Lock it down
   behind basic auth / IP allow-list in Caddy.

---

## 4. Database (Rabhana Postgres)

For completeness — the API's database is a Coolify-managed Postgres, not
something you set up by hand.

| Field | Value |
|-------|-------|
| Container | `c127wknohfrrnco2sje5wmm8` (`postgres:18-alpine`) |
| In-cluster host | `c127wknohfrrnco2sje5wmm8` (same as container name) on the `coolify` network |
| Port | `5432` |
| Public exposure | `c127wknohfrrnco2sje5wmm8-proxy` (`nginx:stable-alpine`) publishes `5432` on the host — keep this firewalled to known IPs |
| Volume | `postgres-data-c127wknohfrrnco2sje5wmm8` |
| `DATABASE_URL` | `postgres://postgres:<password>@c127wknohfrrnco2sje5wmm8:5432/postgres?sslmode=require` |

Coolify takes nightly logical backups (`pg_dump`) into `/data/coolify/backups/`
— configurable on the database's **Backups** tab.

Run migrations against it from your laptop (no need to copy migrations into
the image):

```bash
goose -dir db/migrations postgres \
  "postgres://postgres:<password>@194.163.180.176:5432/postgres?sslmode=require" up
```

---

## 5. End-to-end checklist for a fresh deploy

When deploying the API to a **new environment** (e.g. staging), in order:

1. **DNS**: A records for `api.<env>.example.com` and `storage.<env>.example.com`
   pointing at the new VM.
2. **VM**: Coolify installed; `coolify-proxy` running on 80/443.
3. **MinIO**: container running with rotated root creds (§3.4); `auction-images`
   and `documents` buckets created (§3.5); per-app key created (§3.6); domain
   `storage.<env>.example.com` routed to it via Caddy.
4. **Postgres**: managed Postgres provisioned in Coolify; password recorded.
5. **Source**: GitHub App or Deploy Key configured in Coolify Sources (§2.1).
6. **Application**: created from this repo, Dockerfile build pack, port 8080
   (§2.2); domain `https://api.<env>.example.com` set (§2.3); env vars filled
   (§2.4); `firebase.json` mounted at `/secrets/firebase.json` (§2.5).
7. **Migrations**: run `goose ... up` against the new Postgres.
8. **Deploy**: trigger first deploy; wait for health green.
9. **Smoke test**:
   ```bash
   curl -fsS https://api.<env>.example.com/health
   curl -fsS https://storage.<env>.example.com/minio/health/live
   ```
10. **Auto-deploy**: enable webhook in the Application (§2.6) once smoke passes.

---

## 6. Quick reference — SSH & ops

```bash
# Connect (sshpass for scripts; for humans, drop your pubkey into ~/.ssh/authorized_keys)
sshpass -p '<vm-password>' ssh admin@194.163.180.176

# What's running
sudo docker ps

# Tail an app's logs (use the actual container name from docker ps)
sudo docker logs -f --tail 200 nihxoxm8rqp39ssui0qk7e2e-163807610453

# Restart MinIO
sudo docker restart minio

# Pull a fresh image and redeploy via Coolify CLI button — or from the UI
```

---

*Last verified against the running stack on 2026-05-18.*
