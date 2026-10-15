#!/bin/bash
# Run as root with a root-only payload directory containing agent sources,
# config.json and the two public keys. Never installs or upgrades packages.
set -euo pipefail
umask 077
[[ $(id -u) == 0 && $# == 1 ]] || { printf 'root and one payload directory required\n' >&2; exit 1; }
payload=$(realpath -- "$1")
[[ -d "$payload" && ! -L "$1" ]]
[[ $(stat -c %u "$payload") == 0 ]]
[[ -z $(find "$payload" \( ! -user root -o -perm /022 -o -type l \) -print -quit) ]] || { printf 'payload must be protected and root-owned\n' >&2; exit 1; }
for file in host_agent.py coolify_native_controller.py coolify_native_engine.py systemd/sifto-coolify-native-controller.service maintenance/coolify_systemd.py maintenance/coolify_native_protocol.py maintenance/coolify_cleanup.py maintenance/coolify_native_owner.py maintenance/__init__.py maintenance/plan.py maintenance/host.py maintenance/coolify_host.py maintenance/coolify_plan.py maintenance/coolify_rehearsal.py maintenance/coolify_evidence.py maintenance/coolify_docker.py maintenance/coolify_rehearsal_engine.py maintenance/coolify-public-manifest.json maintenance/coolify_probe.php config.json scan.pub apply.pub; do
  [[ -f "$payload/$file" && ! -L "$payload/$file" && $(stat -c %h "$payload/$file") == 1 ]] || { printf 'invalid payload\n' >&2; exit 1; }
done
scan_fingerprint=$(ssh-keygen -l -f "$payload/scan.pub" | awk '{print $2}')
apply_fingerprint=$(ssh-keygen -l -f "$payload/apply.pub" | awk '{print $2}')
[[ "$scan_fingerprint" != "$apply_fingerprint" ]] || { printf 'gateway keys must be distinct\n' >&2; exit 1; }
# Reuse only identities created by this installer, with no other groups or sudo
# commands. Validate before replacing any executable or configuration.
for gateway in scan apply; do
  account="sifto-maint-$gateway"
  home="/var/lib/sifto-maintenance-ssh/$gateway"
  marker="/etc/sifto-maintenance/$gateway.account"
  if id "$account" >/dev/null 2>&1; then
    [[ $(id -u "$account") -ge 100 && $(id -u "$account") -lt 1000 ]]
    [[ $(getent passwd "$account" | cut -d: -f6) == "$home" ]]
    [[ $(id -gn "$account") == "$account" && $(id -nG "$account") == "$account" ]]
    [[ -f "$marker" && ! -L "$marker" && $(stat -c %u "$marker") == 0 && $(stat -c %a "$marker") == 600 ]]
    [[ $(cat "$marker") == "$(id -u "$account"):$(id -g "$account")" ]]
    COLUMNS=1000 sudo -l -U "$account" | /usr/bin/python3 -I -c '
import sys
expected = "(root) NOPASSWD: /usr/bin/python3 -I /opt/sifto-maintenance/host_agent.py " + sys.argv[1] + "-gateway"
commands = [line.strip() for line in sys.stdin if line.lstrip().startswith("(")]
assert commands == [expected], "unexpected existing sudo grants"
' "$gateway"
  fi
done
/usr/bin/python3 -I -c 'from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey'
[[ $(dpkg --print-architecture) == arm64 ]]
. /etc/os-release
[[ "$ID" == ubuntu && "$VERSION_ID" == 24.04 ]]
for name in scan apply; do
  ssh-keygen -l -f "$payload/$name.pub" >/dev/null
  [[ $(wc -l < "$payload/$name.pub") == 1 ]]
done
for path in /opt/sifto-maintenance /opt/sifto-maintenance/maintenance /opt/sifto-maintenance/systemd /etc/sifto-maintenance /var/lib/sifto-maintenance /var/lib/sifto-maintenance/plans /var/lib/sifto-maintenance-ssh /var/lib/sifto-maintenance/coolify-native-request /var/lib/sifto-maintenance/coolify-native-state /var/lib/sifto-maintenance/coolify-native-runs /var/lib/sifto-maintenance/native-systemd-controller; do
  [[ ! -L "$path" ]] || { printf 'unexpected destination symlink\n' >&2; exit 1; }
  if [[ -e "$path" ]]; then
    [[ -d "$path" && $(stat -c %u "$path") == 0 ]] || { printf 'unexpected destination ownership\n' >&2; exit 1; }
  fi
done
install -d -o root -g root -m 0700 /var/lib/sifto-maintenance /var/lib/sifto-maintenance/plans
if [[ ! -e /var/lib/sifto-maintenance/maintenance.lock ]]; then
  install -o root -g root -m 0600 /dev/null /var/lib/sifto-maintenance/maintenance.lock
fi
[[ ! -L /var/lib/sifto-maintenance/maintenance.lock && $(stat -c %u /var/lib/sifto-maintenance/maintenance.lock) == 0 ]]
exec 9<>/var/lib/sifto-maintenance/maintenance.lock
flock -n 9 || { printf 'maintenance is running; install refused\n' >&2; exit 1; }
install -d -o root -g root -m 0755 /opt/sifto-maintenance /opt/sifto-maintenance/maintenance /opt/sifto-maintenance/systemd
install -o root -g root -m 0644 "$payload/host_agent.py" /opt/sifto-maintenance/host_agent.py
for file in coolify_native_controller.py coolify_native_engine.py; do
  install -o root -g root -m 0644 "$payload/$file" "/opt/sifto-maintenance/$file"
done
install -o root -g root -m 0644 "$payload/systemd/sifto-coolify-native-controller.service" /opt/sifto-maintenance/systemd/sifto-coolify-native-controller.service
for file in __init__.py plan.py host.py coolify_host.py coolify_plan.py coolify_rehearsal.py coolify_evidence.py coolify_docker.py coolify_rehearsal_engine.py coolify_systemd.py coolify_native_protocol.py coolify_cleanup.py coolify_native_owner.py coolify-public-manifest.json coolify_probe.php; do
  install -o root -g root -m 0644 "$payload/maintenance/$file" "/opt/sifto-maintenance/maintenance/$file"
done
install -d -o root -g root -m 0700 /etc/sifto-maintenance /var/lib/sifto-maintenance /var/lib/sifto-maintenance/plans
install -d -o root -g root -m 0700 /var/lib/sifto-maintenance/coolify-native-request /var/lib/sifto-maintenance/coolify-native-state /var/lib/sifto-maintenance/coolify-native-runs /var/lib/sifto-maintenance/native-systemd-controller
# Hash the trusted installer payload, never a restored archive or adjacent manifest.
/usr/bin/python3 -I -B - "$payload" <<'PY'
import hashlib, json, os, pathlib, stat, sys
root = pathlib.Path(sys.argv[1])
names = ('host_agent.py', 'coolify_native_controller.py', 'coolify_native_engine.py',
    'maintenance/__init__.py', 'maintenance/plan.py', 'maintenance/host.py',
    'maintenance/coolify_host.py', 'maintenance/coolify_plan.py',
    'maintenance/coolify_rehearsal.py', 'maintenance/coolify_evidence.py',
    'maintenance/coolify_docker.py', 'maintenance/coolify_rehearsal_engine.py',
    'maintenance/coolify_systemd.py', 'maintenance/coolify_native_protocol.py',
    'maintenance/coolify_cleanup.py', 'maintenance/coolify_native_owner.py',
    'maintenance/coolify-public-manifest.json', 'maintenance/coolify_probe.php',
    'systemd/sifto-coolify-native-controller.service')
document = {'version': 1, 'sources': {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in names}}
directory = os.open('/etc/sifto-maintenance', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
fd = os.open('coolify-native-sources.json', os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW | os.O_NONBLOCK,
             0o600, dir_fd=directory)
try:
    info = os.fstat(fd)
    assert stat.S_ISREG(info.st_mode) and info.st_uid == info.st_gid == 0 and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600
    os.ftruncate(fd, 0)
    raw = json.dumps(document, sort_keys=True, separators=(',', ':')).encode()
    while raw:
        raw = raw[os.write(fd, raw):]
    os.fsync(fd)
    os.fsync(directory)
finally:
    os.close(fd)
    os.close(directory)
PY
config_temp=$(mktemp /etc/sifto-maintenance/config.XXXXXXXX)
trap 'rm -f -- "$config_temp"' EXIT
install -o root -g root -m 0600 "$payload/config.json" "$config_temp"
/usr/bin/python3 -I -B - "$config_temp" <<'PY'
import sys
sys.dont_write_bytecode = True
sys.pycache_prefix = "/dev/null"
sys.path.insert(0, "/opt/sifto-maintenance")
from maintenance.host import load_config
load_config(sys.argv[1])
PY
mv -f -- "$config_temp" /etc/sifto-maintenance/config.json
install -d -o root -g root -m 0755 /var/lib/sifto-maintenance-ssh
for gateway in scan apply; do
  account="sifto-maint-$gateway"
  home="/var/lib/sifto-maintenance-ssh/$gateway"
  if id "$account" >/dev/null 2>&1; then
    : # Existing identity was validated before installation.
  else
    useradd --system --home-dir "$home" --shell /bin/sh "$account"
  fi
  passwd -l "$account" >/dev/null
  [[ ! -L "$home" && ! -L "$home/.ssh" && ! -L "$home/.ssh/authorized_keys" ]]
  install -d -o root -g root -m 0755 "$home" "$home/.ssh"
  key_temp=$(mktemp "$home/.ssh/authorized_keys.XXXXXXXX")
  printf 'restrict,command="sudo -n /usr/bin/python3 -I /opt/sifto-maintenance/host_agent.py %s-gateway" %s\n' \
    "$gateway" "$(cat "$payload/$gateway.pub")" > "$key_temp"
  chmod 0644 "$key_temp"
  mv -f -- "$key_temp" "$home/.ssh/authorized_keys"
done
sudoers_temp=$(mktemp /etc/sudoers.d/sifto-maintenance.XXXXXXXX)
cat > "$sudoers_temp" <<'SUDOERS'
Defaults:sifto-maint-scan env_reset
Defaults:sifto-maint-apply env_reset
Defaults:sifto-maint-scan env_keep += "SSH_ORIGINAL_COMMAND"
Defaults:sifto-maint-apply env_keep += "SSH_ORIGINAL_COMMAND"
sifto-maint-scan ALL=(root) NOPASSWD: /usr/bin/python3 -I /opt/sifto-maintenance/host_agent.py scan-gateway
sifto-maint-apply ALL=(root) NOPASSWD: /usr/bin/python3 -I /opt/sifto-maintenance/host_agent.py apply-gateway
SUDOERS
chmod 0440 "$sudoers_temp"
visudo -cf "$sudoers_temp" >/dev/null
mv -f -- "$sudoers_temp" /etc/sudoers.d/sifto-maintenance
for gateway in scan apply; do
  account="sifto-maint-$gateway"
  printf '%s:%s\n' "$(id -u "$account")" "$(id -g "$account")" > "/etc/sifto-maintenance/$gateway.account"
  chmod 0600 "/etc/sifto-maintenance/$gateway.account"
done
printf 'maintenance gateways installed; no package changes performed\n'
