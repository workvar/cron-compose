-- Job templates: a starting point for a new job.
--
-- Two kinds live in one table. Built-ins ship with CronCompose and are seeded here;
-- they are read-only so an upgrade can replace them without destroying anyone's work.
-- Anything a user saves has builtin = false and belongs to them to edit or delete.
--
-- Built-in scripts are deliberately conservative: they use POSIX-ish shell, avoid
-- anything destructive without an explicit path, and are written to be read and edited
-- rather than run blind.

begin;

create table if not exists job_templates (
  id            text primary key,
  name          text not null,
  description   text not null default '',
  category      text not null default 'general',
  interpreter   text not null default 'bash',
  script_body   text not null,
  schedule_cron text not null default '0 * * * *',
  timezone      text not null default 'UTC',
  env           jsonb not null default '{}'::jsonb,
  builtin       boolean not null default false,
  created_by    text references users(id) on delete set null,
  created_at    timestamptz not null default now()
);

create index if not exists job_templates_category_idx on job_templates (category, name);

-- Seeded by fixed id so a re-run updates in place instead of duplicating.
insert into job_templates (id, name, description, category, interpreter, script_body, schedule_cron, builtin)
values
  ('tpl_disk_space', 'Disk space check', 'Fail when any mounted filesystem is over a threshold, so the notification arrives before the disk fills.', 'monitoring', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

# Fail the run when any real filesystem crosses this percentage. Failing is the point:
# that is what triggers the notification.
THRESHOLD=${THRESHOLD:-85}

over=0
while read -r pct mount; do
  pct=${pct%\%}
  if [ "$pct" -ge "$THRESHOLD" ]; then
    echo "WARN  $mount is at ${pct}% (threshold ${THRESHOLD}%)"
    over=1
  else
    echo "ok    $mount is at ${pct}%"
  fi
done < <(df -P -x tmpfs -x devtmpfs -x overlay | awk 'NR>1 {print $5, $6}')

exit "$over"
$script$, '0 * * * *', true),

  ('tpl_postgres_backup', 'PostgreSQL backup', 'Dump a database with pg_dump, compress it, and keep the last N days. Set PGDATABASE and BACKUP_DIR before enabling.', 'backup', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

# Required: PGDATABASE. Everything else has a default. Credentials belong in a
# CronCompose secret (PGPASSWORD), not in this script.
: "${PGDATABASE:?set PGDATABASE}"
BACKUP_DIR=${BACKUP_DIR:-/var/backups/postgres}
KEEP_DAYS=${KEEP_DAYS:-7}

mkdir -p "$BACKUP_DIR"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out="$BACKUP_DIR/${PGDATABASE}-${stamp}.sql.gz"

# Write to a temp name first so a failed dump never leaves a truncated file that
# looks like a valid backup.
pg_dump --no-owner --no-privileges "$PGDATABASE" | gzip -9 > "$out.partial"
mv "$out.partial" "$out"
echo "wrote $out ($(du -h "$out" | cut -f1))"

# Prune only files this template created, matched by name, never by mtime alone.
find "$BACKUP_DIR" -name "${PGDATABASE}-*.sql.gz" -type f -mtime "+${KEEP_DAYS}" -print -delete
$script$, '0 3 * * *', true),

  ('tpl_docker_prune', 'Docker cleanup', 'Reclaim space from stopped containers, dangling images, and unused build cache. Leaves named volumes alone.', 'maintenance', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

echo "before:"
docker system df

# Volumes are excluded on purpose. Pruning them is how people lose databases.
docker container prune -f
docker image prune -f
docker builder prune -f --keep-storage 5GB

echo
echo "after:"
docker system df
$script$, '0 4 * * 0', true),

  ('tpl_cert_expiry', 'TLS certificate expiry', 'Fail when a certificate is close to expiring, giving you time to renew.', 'monitoring', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

# Space-separated host:port list.
HOSTS=${HOSTS:-example.com:443}
WARN_DAYS=${WARN_DAYS:-14}

now=$(date -u +%s)
bad=0

for hp in $HOSTS; do
  host=${hp%%:*}
  port=${hp##*:}
  [ "$port" = "$host" ] && port=443

  end=$(echo | openssl s_client -servername "$host" -connect "$host:$port" 2>/dev/null \
        | openssl x509 -noout -enddate | cut -d= -f2)
  if [ -z "$end" ]; then
    echo "ERROR $hp: could not read a certificate"
    bad=1
    continue
  fi

  end_s=$(date -u -d "$end" +%s 2>/dev/null || date -u -j -f "%b %e %T %Y %Z" "$end" +%s)
  days=$(( (end_s - now) / 86400 ))
  if [ "$days" -le "$WARN_DAYS" ]; then
    echo "WARN  $hp expires in ${days}d ($end)"
    bad=1
  else
    echo "ok    $hp expires in ${days}d"
  fi
done

exit "$bad"
$script$, '0 8 * * *', true),

  ('tpl_healthcheck', 'HTTP health check', 'Fail when an endpoint stops returning the expected status, with the response body in the run log.', 'monitoring', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

URL=${URL:?set URL}
EXPECT=${EXPECT:-200}
TIMEOUT=${TIMEOUT:-10}

body=$(mktemp)
trap 'rm -f "$body"' EXIT

code=$(curl -sS -o "$body" -w '%{http_code}' --max-time "$TIMEOUT" "$URL" || echo 000)
echo "GET $URL -> $code"

if [ "$code" != "$EXPECT" ]; then
  echo "--- response ---"
  head -c 2000 "$body"
  exit 1
fi
$script$, '*/5 * * * *', true),

  ('tpl_log_rotate', 'Prune old log files', 'Delete log files older than a cutoff under one directory. Prints what it removes.', 'maintenance', 'bash',
$script$#!/usr/bin/env bash
set -euo pipefail

# No default for the directory on purpose: a typo should not delete the wrong tree.
LOG_DIR=${LOG_DIR:?set LOG_DIR}
KEEP_DAYS=${KEEP_DAYS:-30}
PATTERN=${PATTERN:-*.log}

[ -d "$LOG_DIR" ] || { echo "no such directory: $LOG_DIR"; exit 1; }

echo "pruning $PATTERN older than ${KEEP_DAYS}d in $LOG_DIR"
find "$LOG_DIR" -type f -name "$PATTERN" -mtime "+${KEEP_DAYS}" -print -delete
$script$, '30 2 * * *', true)
on conflict (id) do update set
  name = excluded.name,
  description = excluded.description,
  category = excluded.category,
  interpreter = excluded.interpreter,
  script_body = excluded.script_body,
  schedule_cron = excluded.schedule_cron,
  builtin = true;

commit;
