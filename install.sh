#!/usr/bin/env bash
# Lithium first-time setup: asks for the admin account and email settings,
# writes them to .env, then (optionally) builds and starts the container.
#
#   ./install.sh          set up, or choose what to do with an existing install
#   ./install.sh update   pull the latest version from GitHub, rebuild and restart
set -euo pipefail

cd "$(dirname "$0")"
ENV_FILE=".env"

# ── Styling ────────────────────────────────────────────────────────────────
if [[ -t 1 ]] && [[ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]]; then
	BOLD=$'\e[1m'; DIM=$'\e[2m'; RED=$'\e[31m'; GREEN=$'\e[32m'
	YELLOW=$'\e[33m'; CYAN=$'\e[36m'; RESET=$'\e[0m'
	GRADIENT=($'\e[38;5;51m' $'\e[38;5;45m' $'\e[38;5;39m' $'\e[38;5;63m' $'\e[38;5;99m' $'\e[38;5;135m')
else
	BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; CYAN=""; RESET=""
	GRADIENT=("" "" "" "" "" "")
fi

banner() {
	local lines=(
		"██╗     ██╗████████╗██╗  ██╗██╗██╗   ██╗███╗   ███╗"
		"██║     ██║╚══██╔══╝██║  ██║██║██║   ██║████╗ ████║"
		"██║     ██║   ██║   ███████║██║██║   ██║██╔████╔██║"
		"██║     ██║   ██║   ██╔══██║██║██║   ██║██║╚██╔╝██║"
		"███████╗██║   ██║   ██║  ██║██║╚██████╔╝██║ ╚═╝ ██║"
		"╚══════╝╚═╝   ╚═╝   ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝     ╚═╝"
	)
	echo
	for i in "${!lines[@]}"; do
		printf '  %s%s%s\n' "${GRADIENT[$i]}" "${lines[$i]}" "$RESET"
	done
	printf '\n  %s┌────┐%s\n' "$DIM" "$RESET"
	printf '  %s│ %s3%s  │%s  %sFirst-time setup%s\n' "$DIM" "$RESET$CYAN" "$RESET$DIM" "$RESET" "$BOLD" "$RESET"
	printf '  %s│ %s%sLi%s │%s  %sAdmin account · Email · Start%s\n' "$DIM" "$RESET" "$BOLD" "$RESET$DIM" "$RESET" "$DIM" "$RESET"
	printf '  %s└────┘%s\n\n' "$DIM" "$RESET"
}

section() { printf '\n%s━━ %s %s\n' "$CYAN" "$1" "$RESET"; }
info()    { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }
ok()      { printf '  %s✔%s %s\n' "$GREEN" "$RESET" "$1"; }
warn()    { printf '  %s!%s %s\n' "$YELLOW" "$RESET" "$1"; }
err()     { printf '  %s✘%s %s\n' "$RED" "$RESET" "$1" >&2; }
die()     { err "$1"; exit 1; }

# ── Prompts ────────────────────────────────────────────────────────────────
# Values are written to .env in single quotes so $, \ and spaces are kept
# literally; a single quote itself can't be represented, so it's refused.
safe() { [[ "$1" != *"'"* ]]; }

# ask VAR "Question" [default]
ask() {
	local __var=$1 prompt=$2 default=${3-} reply
	while true; do
		if [[ -n "$default" ]]; then
			read -r -p "  $prompt ${DIM}[$default]${RESET}: " reply </dev/tty
			reply=${reply:-$default}
		else
			read -r -p "  $prompt: " reply </dev/tty
		fi
		if ! safe "$reply"; then err "Single quotes (') aren't allowed."; continue; fi
		printf -v "$__var" '%s' "$reply"
		return
	done
}

# ask_required VAR "Question" [default]
ask_required() {
	while true; do
		ask "$@"
		[[ -n "${!1}" ]] && return
		err "This one is required."
	done
}

# ask_yn "Question" default(y|n) → exit status 0 for yes
ask_yn() {
	local prompt=$1 default=$2 hint reply
	[[ "$default" == y ]] && hint="Y/n" || hint="y/N"
	while true; do
		read -r -p "  $prompt ${DIM}[$hint]${RESET}: " reply </dev/tty
		reply=${reply:-$default}
		case "$(lower "$reply")" in
			y|yes) return 0 ;;
			n|no)  return 1 ;;
			*) err "Please answer y or n." ;;
		esac
	done
}

# ask_secret VAR "Question" min_length allow_empty(0|1)
ask_secret() {
	local __var=$1 prompt=$2 min=$3 allow_empty=$4 first second
	while true; do
		read -r -s -p "  $prompt: " first </dev/tty; echo
		if [[ -z "$first" && "$allow_empty" == 1 ]]; then printf -v "$__var" ''; return; fi
		if (( ${#first} < min )); then err "Must be at least $min characters."; continue; fi
		if ! safe "$first"; then err "Single quotes (') aren't allowed."; continue; fi
		read -r -s -p "  Confirm: " second </dev/tty; echo
		if [[ "$first" != "$second" ]]; then err "They don't match, try again."; continue; fi
		printf -v "$__var" '%s' "$first"
		return
	done
}

# lower TEXT → TEXT in lower case. (Not ${x,,}: macOS ships bash 3.2.)
lower() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

valid_email() { [[ "$1" =~ ^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$ ]]; }

gen_password() {
	LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 20 || true
}

# env_get KEY → value of KEY from .env, with surrounding quotes removed
env_get() {
	local line
	line=$(grep -E "^$1=" "$ENV_FILE" | tail -n 1) || true
	line=${line#*=}
	line=${line#[\'\"]}; line=${line%[\'\"]}
	printf '%s' "$line"
}

# start_lithium [--pull] → builds the image, (re)starts the container and
# waits for the health check. Sets started=1 when it comes up healthy.
started=0
start_lithium() {
	section "Building and starting"
	# Baked into the image so Settings → Updates knows which version is running.
	LITHIUM_COMMIT=$(git rev-parse HEAD 2>/dev/null || true)
	export LITHIUM_COMMIT
	if [[ "${1-}" == --pull ]]; then
		"${DOCKER[@]}" compose build --pull
		"${DOCKER[@]}" compose up -d
	else
		"${DOCKER[@]}" compose up -d --build
	fi
	info "Waiting for the health check..."
	local cid status=unknown
	cid=$("${DOCKER[@]}" compose ps -q lithium)
	for _ in $(seq 1 30); do
		status=$("${DOCKER[@]}" inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null || echo unknown)
		[[ "$status" == healthy ]] && break
		sleep 2
	done
	if [[ "$status" == healthy ]]; then
		ok "Lithium is running."
		started=1
	else
		warn "It hasn't reported healthy yet. Check: ${DOCKER[*]} compose logs -f lithium"
	fi
}

# update_lithium → pulls the latest commits from GitHub (fast-forward only, so
# local changes are never overwritten), then rebuilds and restarts.
update_lithium() {
	(( HAVE_DOCKER )) || die "Docker isn't usable (see above), so Lithium can't be rebuilt."
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 \
		|| die "This folder isn't a git clone, so it can't pull updates. Clone it with: git clone https://github.com/Ben-wh/Lithium.git"
	git rev-parse --abbrev-ref '@{upstream}' >/dev/null 2>&1 \
		|| die "The current branch doesn't track a GitHub branch. Try: git checkout main"

	section "Checking GitHub for updates"
	git fetch --quiet || die "Couldn't fetch from GitHub. Check the server's internet connection."
	local behind ahead
	behind=$(git rev-list --count 'HEAD..@{upstream}')
	ahead=$(git rev-list --count '@{upstream}..HEAD')

	if (( behind == 0 )); then
		ok "Already up to date ($(git rev-parse --short HEAD))."
		ask_yn "Rebuild and restart anyway?" n || { echo; exit 0; }
	else
		info "$behind new commit(s):"
		git log --format='    %h  %s' 'HEAD..@{upstream}' | head -n 20
		(( behind > 20 )) && info "    ...and $((behind - 20)) more"
		echo
		if (( ahead > 0 )); then
			die "This copy has $ahead local commit(s) that aren't on GitHub, so it can't update automatically. Merge by hand with: git pull"
		fi
		if ! git diff --quiet HEAD -- 2>/dev/null; then
			warn "Some tracked files have been changed here:"
			git status --short --untracked-files=no | sed 's/^/    /'
			die "Commit or undo those changes (git stash), then run ./install.sh update again."
		fi
		ask_yn "Update to $(git rev-parse --short '@{upstream}') and restart Lithium?" y || { info "Nothing changed."; exit 0; }
		# git replaces files rather than editing them in place, so this running
		# script carries on safely even if the update changes it.
		git merge --ff-only --quiet '@{upstream}' || die "git couldn't apply the update. Try: git pull"
		ok "Updated to $(git rev-parse --short HEAD)."
	fi

	start_lithium
	section "Done"
	if (( started )); then
		printf '  Open %s%s%s\n' "$BOLD" "$(env_get LITHIUM_BASE_URL)" "$RESET"
	fi
	info "Database changes run automatically on start. Logs: ${DOCKER[*]} compose logs -f lithium"
	echo
	exit 0
}

# ── Preflight ──────────────────────────────────────────────────────────────
banner

[[ -r /dev/tty ]] || die "This script is interactive; run it from a terminal."

HAVE_DOCKER=0
DOCKER=(docker)
if ! command -v docker >/dev/null || ! docker compose version >/dev/null 2>&1; then
	warn "Docker with the compose plugin wasn't found. You can still write .env now and start Lithium later."
elif docker_err=$(docker info 2>&1); then
	HAVE_DOCKER=1
elif [[ "$(lower "$docker_err")" == *"permission denied"* ]]; then
	warn "Your user can't talk to Docker (not in the 'docker' group)."
	info "To fix it for good:  sudo usermod -aG docker \$USER  then log out and back in."
	if command -v sudo >/dev/null && ask_yn "Use sudo for Docker this time instead?" y; then
		DOCKER=(sudo docker)
		"${DOCKER[@]}" info >/dev/null 2>&1 && HAVE_DOCKER=1 || warn "sudo docker didn't work either; Lithium won't be started."
	fi
else
	warn "Docker is installed but the daemon isn't running. Try: sudo systemctl start docker"
fi

case "${1-}" in
	"") ;;
	update)
		[[ -f "$ENV_FILE" ]] || die "No $ENV_FILE yet. Run ./install.sh first to set Lithium up."
		update_lithium
		;;
	*) die "Unknown option: $1  (usage: ./install.sh [update])" ;;
esac

if [[ -f "$ENV_FILE" ]]; then
	section "Existing install"
	info "Found $ENV_FILE (admin: $(env_get LITHIUM_ADMIN_EMAIL), URL: $(env_get LITHIUM_BASE_URL))"
	echo
	info "  1) Update from GitHub, then rebuild and restart  (data is kept)"
	info "  2) Rebuild and restart with these settings  (data is kept)"
	info "  3) Reconfigure: answer the questions again and write a new $ENV_FILE"
	info "  4) Quit"
	while true; do
		ask choice "Choose 1-4" "1"
		case "$choice" in
			1|2|3|4) break ;;
			*) err "Enter 1, 2, 3 or 4." ;;
		esac
	done

	case "$choice" in
		1) update_lithium ;;
		2)
			(( HAVE_DOCKER )) || die "Docker isn't usable (see above), so Lithium can't be rebuilt."
			pull=""
			ask_yn "Also pull the latest base images (Go, Node, etc.)?" n && pull=--pull
			start_lithium $pull
			section "Done"
			if (( started )); then
				printf '  Open %s%s%s\n' "$BOLD" "$(env_get LITHIUM_BASE_URL)" "$RESET"
			fi
			info "Logs: ${DOCKER[*]} compose logs -f lithium"
			echo
			exit 0
			;;
		4) info "Nothing changed."; exit 0 ;;
	esac

	info "The admin account is only created on the very first start. If Lithium has"
	info "already run, changing it here won't change the existing admin's login."
fi

# ── Admin account ──────────────────────────────────────────────────────────
section "Admin account"
info "This is the first user. You sign in with the email address."
while true; do
	ask_required ADMIN_EMAIL "Admin email"
	valid_email "$ADMIN_EMAIL" && break
	err "That doesn't look like an email address."
done
ask_required ADMIN_NAME "Display name" "Administrator"

if ask_yn "Generate a strong password for you?" n; then
	ADMIN_PASSWORD=$(gen_password)
	GENERATED_PASSWORD=1
	ok "Password generated. It's shown once at the end, so note it down."
else
	ask_secret ADMIN_PASSWORD "Admin password (8+ characters)" 8 0
	GENERATED_PASSWORD=0
fi

# ── Web address ────────────────────────────────────────────────────────────
section "Web address"
while true; do
	ask_required PORT "Port to listen on" "8080"
	[[ "$PORT" =~ ^[0-9]+$ ]] && (( PORT >= 1 && PORT <= 65535 )) && break
	err "Enter a port number between 1 and 65535."
done

if ask_yn "Will people reach it over HTTPS through a reverse proxy (Caddy, Traefik, nginx)?" n; then
	SECURE_COOKIES=true
	TRUST_PROXY=true
	ask_required BASE_URL "Public URL" "https://help.example.com"
else
	SECURE_COOKIES=false
	TRUST_PROXY=false
	ask_required BASE_URL "URL people will use" "http://localhost:$PORT"
fi
BASE_URL=${BASE_URL%/}

# ── Email ──────────────────────────────────────────────────────────────────
section "Email notifications"
SMTP_HOST="" SMTP_PORT="" SMTP_USERNAME="" SMTP_PASSWORD="" SMTP_FROM="" SMTP_TLS=""
info "You can set email up here, or later in Lithium under Settings → Email."
info "Setting it here fixes it in $ENV_FILE, and the Email page then shows it read-only."
if ask_yn "Set up email now?" n; then
	ask_required SMTP_HOST "SMTP host" "smtp.example.com"

	echo
	info "Encryption:"
	info "  1) STARTTLS  usually port 587 (most providers)"
	info "  2) TLS       usually port 465"
	info "  3) None      local test servers only, e.g. Mailpit"
	while true; do
		ask choice "Choose 1-3" "1"
		case "$choice" in
			1) SMTP_TLS=starttls; default_port=587; break ;;
			2) SMTP_TLS=tls;      default_port=465; break ;;
			3) SMTP_TLS=none;     default_port=1025; break ;;
			*) err "Enter 1, 2 or 3." ;;
		esac
	done

	while true; do
		ask_required SMTP_PORT "SMTP port" "$default_port"
		[[ "$SMTP_PORT" =~ ^[0-9]+$ ]] && break
		err "Enter a number."
	done

	if [[ "$SMTP_TLS" == none ]]; then
		info "No encryption, so no login is sent."
	else
		ask SMTP_USERNAME "SMTP username (blank for none)"
		if [[ -n "$SMTP_USERNAME" ]]; then
			ask_secret SMTP_PASSWORD "SMTP password" 1 0
		fi
	fi

	from_default=""
	valid_email "$SMTP_USERNAME" && from_default=$SMTP_USERNAME
	while true; do
		ask_required SMTP_FROM "From address" "$from_default"
		valid_email "$SMTP_FROM" && break
		err "That doesn't look like an email address."
	done
else
	info "Skipped. An admin can set it up any time in Settings → Email."
fi

# ── Summary ────────────────────────────────────────────────────────────────
section "Summary"
printf '  %-16s %s\n' "Admin" "$ADMIN_NAME <$ADMIN_EMAIL>"
printf '  %-16s %s\n' "URL" "$BASE_URL"
printf '  %-16s %s\n' "Port" "$PORT"
printf '  %-16s %s\n' "HTTPS cookies" "$SECURE_COOKIES"
if [[ -n "$SMTP_HOST" ]]; then
	printf '  %-16s %s\n' "SMTP" "$SMTP_HOST:$SMTP_PORT ($SMTP_TLS)"
	printf '  %-16s %s\n' "From" "$SMTP_FROM"
	[[ -n "$SMTP_USERNAME" ]] && printf '  %-16s %s\n' "SMTP login" "$SMTP_USERNAME"
else
	printf '  %-16s %s\n' "Email" "off"
fi
echo
ask_yn "Write these settings to $ENV_FILE?" y || { info "Nothing written."; exit 0; }

# ── Write .env ─────────────────────────────────────────────────────────────
if [[ -f "$ENV_FILE" ]]; then
	backup="$ENV_FILE.bak.$(date +%Y%m%d%H%M%S)"
	cp -p "$ENV_FILE" "$backup"
	chmod 600 "$backup"
	ok "Old settings backed up to $backup"
fi

tmp=$(mktemp "$ENV_FILE.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"
{
	echo "# Written by install.sh on $(date). Holds passwords: keep it private."
	echo "LITHIUM_PORT='$PORT'"
	echo "LITHIUM_ADMIN_EMAIL='$ADMIN_EMAIL'"
	echo "LITHIUM_ADMIN_NAME='$ADMIN_NAME'"
	echo "LITHIUM_ADMIN_PASSWORD='$ADMIN_PASSWORD'"
	echo "LITHIUM_SECURE_COOKIES='$SECURE_COOKIES'"
	echo "LITHIUM_TRUST_PROXY='$TRUST_PROXY'"
	echo "LITHIUM_BASE_URL='$BASE_URL'"
	echo
	echo "# Email notifications (off unless LITHIUM_SMTP_HOST is set)."
	echo "LITHIUM_SMTP_HOST='$SMTP_HOST'"
	echo "LITHIUM_SMTP_PORT='$SMTP_PORT'"
	echo "LITHIUM_SMTP_USERNAME='$SMTP_USERNAME'"
	echo "LITHIUM_SMTP_PASSWORD='$SMTP_PASSWORD'"
	echo "LITHIUM_SMTP_FROM='$SMTP_FROM'"
	echo "LITHIUM_SMTP_TLS='$SMTP_TLS'"
} >"$tmp"
mv "$tmp" "$ENV_FILE"
trap - EXIT
ok "Saved $ENV_FILE (readable only by you)"

# ── Start ──────────────────────────────────────────────────────────────────
if (( HAVE_DOCKER )); then
	echo
	ask_yn "Build and start Lithium now?" y && start_lithium
fi

section "Done"
if (( started )); then
	printf '  Open %s%s%s and sign in as %s%s%s\n' "$BOLD" "$BASE_URL" "$RESET" "$BOLD" "$ADMIN_EMAIL" "$RESET"
else
	info "Start it any time with: ${DOCKER[*]} compose up -d --build"
fi
if (( GENERATED_PASSWORD )); then
	printf '  Admin password: %s%s%s  %s(only shown now)%s\n' "$BOLD$YELLOW" "$ADMIN_PASSWORD" "$RESET" "$DIM" "$RESET"
fi
info "Logs: ${DOCKER[*]} compose logs -f lithium"
echo
