<div align="center">

<img src="web/static/favicon.svg" width="88" height="88" alt="Lithium logo">

# Lithium

**A lightweight, self-hosted helpdesk.**
Tickets, a knowledge base and reports in a single binary with one SQLite file.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![SvelteKit](https://img.shields.io/badge/SvelteKit-Svelte_5-FF3E00?logo=svelte&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)

[Features](#-features) · [Deploy](#-deploy) · [Configuration](#-configuration) · [Operations](#-operations) · [Development](#-development)

</div>

---

## ✨ Features

Lithium has three roles: **users** raise tickets, **agents** work on them, and **admins** run the workspace.

<table>
<tr>
<td width="50%" valign="top">

### 🎫 Tickets
- Categories, priorities and a clear status flow: Unassigned, In progress, Pending, Resolved, Closed
- Replies, **internal notes** for staff, and **@mentions** of people or whole groups
- Image attachments, which can be dragged, chosen or pasted
- **Ticket templates** with extra fields per category (text, number, dropdown, checkbox, date)
- Raise tickets on someone's behalf
- **Linked tickets** and an **incident tree** diagram for major outages

</td>
<td width="50%" valign="top">

### 📚 Knowledge base
- Markdown articles with a toolbar, images, tables and checklists
- Collections, search, drafts and publishing
- Per-article visibility: **Everyone**, **Staff only** or **Admins only**
- Ships with three **How to use Lithium** guides, one each for users, agents and admins

</td>
</tr>
<tr>
<td valign="top">

### 📊 Insight
- **Reports** over 7, 30 or 90 days: volume, first response, time to resolve, workload
- CSV and JSON ticket export
- **User book** listing everyone who raises tickets, with their history
- In-app notifications and batched **email notifications**

</td>
<td valign="top">

### 🔒 Security and admin
- Two-factor authentication (TOTP) with recovery codes, optionally **mandatory**
- **Agent groups** that limit agents to their teams' categories
- **Lockdown mode**: sign everyone else out in an emergency
- **Activity log** of every change and sign-in attempt
- Branding (logo, name, accent colour, background) and a **sensitive mode** banner

</td>
</tr>
</table>

---

## 🚀 Deploy

All you need is **Docker** with the **compose plugin**.

### Option 1: guided installer (recommended)

```sh
git clone https://github.com/Ben-wh/Lithium.git
cd Lithium
./install.sh
```

The installer asks for:

1. **The admin account**: email, name and password. It can generate a strong password for you.
2. **The web address**: the port, and whether you're behind an HTTPS reverse proxy.
3. **Email (optional)**: SMTP details for notification emails. You can also set this up later in the app.

It then writes `.env` (readable only by you), builds the image, starts the container and waits for the health check.

Run `./install.sh` again later to **update from GitHub**, **rebuild and restart** with your existing settings (your data is kept), or to reconfigure.

### Option 2: by hand

```sh
cp .env.example .env
# Edit .env: at minimum set LITHIUM_ADMIN_EMAIL and LITHIUM_ADMIN_PASSWORD
docker compose up -d --build
```

Open **http://localhost:8080** and sign in with the admin email and password.

> [!NOTE]
> The admin account is only created on the **very first start**, when no users exist. Changing `LITHIUM_ADMIN_*` afterwards doesn't change the existing admin's login.

### Running behind HTTPS

Put Lithium behind a reverse proxy such as Caddy, Traefik or nginx, then set:

```ini
LITHIUM_SECURE_COOKIES=true
LITHIUM_TRUST_PROXY=true
LITHIUM_BASE_URL=https://help.example.com
```

For example, the whole Caddy config:

```caddyfile
help.example.com {
	reverse_proxy localhost:8080
}
```

> [!IMPORTANT]
> Only set `LITHIUM_TRUST_PROXY=true` when Lithium is reachable **only** through the proxy. Otherwise clients could fake their IP address in the activity log.

### First steps after deploying

- [ ] Upload your logo and set your company name under **Settings → Appearance**
- [ ] Add categories, and templates for any that need extra details, under **Settings → Tickets**
- [ ] Set up email under **Settings → Email**
- [ ] Turn on mandatory two-factor under **Settings → Users**
- [ ] Create accounts for your agents under **Admin → Users**
- [ ] Read the **How to use Lithium** guides in the knowledge base

---

## 🔧 Configuration

Lithium is configured with environment variables, which `docker compose` reads from `.env`. Everything else is set in the app under **Settings**.

| Variable | Default | Description |
| --- | --- | --- |
| `LITHIUM_ADMIN_EMAIL` | `admin@example.com` | Email of the first admin, created on first start |
| `LITHIUM_ADMIN_PASSWORD` | *(required)* | Password of the first admin |
| `LITHIUM_ADMIN_NAME` | `Administrator` | Display name of the first admin |
| `LITHIUM_PORT` | `8080` | Host port that compose publishes |
| `LITHIUM_BASE_URL` | *(empty)* | Public URL, used for links in emails |
| `LITHIUM_SECURE_COOKIES` | `false` | Set `true` when served over HTTPS |
| `LITHIUM_TRUST_PROXY` | `false` | Trust `X-Forwarded-For` from your reverse proxy |
| `LITHIUM_DATA_DIR` | `/data` in Docker | Where the database, attachments and images live |
| `LITHIUM_DB_PATH` | `$LITHIUM_DATA_DIR/lithium.db` | SQLite database file |
| `LITHIUM_ADDR` | `:8080` | Address the server listens on |

### Email (optional)

Email is off unless `LITHIUM_SMTP_HOST` is set. If you leave these blank, an admin can configure email in **Settings → Email** instead. If you set them here, that page becomes read-only.

| Variable | Default | Description |
| --- | --- | --- |
| `LITHIUM_SMTP_HOST` | *(empty)* | SMTP server, e.g. `smtp.example.com` |
| `LITHIUM_SMTP_PORT` | `587` (`465` for TLS) | SMTP port |
| `LITHIUM_SMTP_TLS` | `starttls` | `starttls`, `tls`, or `none` (local test servers only) |
| `LITHIUM_SMTP_USERNAME` | *(empty)* | Leave blank if the server needs no login |
| `LITHIUM_SMTP_PASSWORD` | *(empty)* | Often an app password rather than your normal one |
| `LITHIUM_SMTP_FROM` | *(empty)* | Plain address, e.g. `help@example.com` |

---

## 🛠 Operations

### Updating

Admins can see whether a newer version is on GitHub under **Settings → Updates**. To install it, run this in the Lithium folder on the server:

```sh
./install.sh update
```

It shows what's new, pulls it from GitHub (fast-forward only, so local changes are never overwritten), then rebuilds and restarts. Your data is kept and database migrations run automatically on start.

To do it by hand instead:

```sh
git pull
LITHIUM_COMMIT=$(git rev-parse HEAD) docker compose up -d --build
```

`LITHIUM_COMMIT` tells Lithium which version it's running. Without it, Settings → Updates can't compare versions.

### Backups

Everything lives in the `lithium-data` Docker volume: the SQLite database, ticket attachments, knowledge base images, avatars and branding. Stop the container briefly for a consistent copy:

```sh
docker compose stop
docker compose cp lithium:/data ./lithium-backup-$(date +%F)
docker compose start
```

### Logs and health

```sh
docker compose logs -f lithium
curl http://localhost:8080/healthz
```

### Emergency commands

These run on the server for when nobody can fix things from the app:

| Command | What it does |
| --- | --- |
| `docker compose exec lithium /lithium reset-2fa <email>` | Turns off two-factor for an account, e.g. an admin who lost their phone and recovery codes |
| `docker compose exec lithium /lithium end-lockdown` | Ends lockdown mode when the admin who started it can't sign in |
| `docker compose exec lithium /lithium seed-example` | Adds the **How to use Lithium** guides to an empty knowledge base |

---

## 💻 Development

**Requirements:** Go 1.26+ and Node 22+.

The Go server embeds the built frontend, so build it once before the first run:

```sh
cd web && npm install && npm run build && cd ..
```

Then run the API and the frontend dev server side by side:

```sh
# Terminal 1: API on :8080 (data in ./data)
LITHIUM_ADMIN_EMAIL=admin@example.com LITHIUM_ADMIN_PASSWORD=change-me-please go run ./cmd/lithium

# Terminal 2: frontend with hot reload, proxying /api to :8080
cd web && npm run dev
```

Open the URL Vite prints (usually **http://localhost:5173**).

### Tests and checks

```sh
go test ./...
cd web && npm run check
```

### Project layout

```text
cmd/lithium/        entry point and server-side CLI commands
internal/
  auth/             passwords and TOTP
  config/           environment configuration
  db/               SQLite connection and migrations
  mail/             SMTP sending and the email outbox
  seed/             the "How to use Lithium" guides
  server/           HTTP handlers, access rules, notifications
  store/            database queries
web/                SvelteKit frontend (Svelte 5, Tailwind 4), built to static files
```
