#!/bin/bash
# Real sudo/sshd boundaries inside an ARM Ubuntu24 disposable container.
set -euo pipefail
umask 077
install -d -m 0700 /root/payload /root/payload/maintenance
cp /fixture/host_agent.py /root/payload/
cp /fixture/maintenance/*.py /root/payload/maintenance/
ssh-keygen -q -t ed25519 -N '' -f /root/payload/scan
ssh-keygen -q -t ed25519 -N '' -f /root/payload/apply
cat > /root/payload/config.json <<'JSON'
{
  "host_id":"fixture",
  "host_role":"application",
  "approved_workflow_sha":"0000000000000000000000000000000000000000",
  "trusted_public_key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  "state_path":"/var/lib/sifto-maintenance",
  "allowed_approvers":[],
  "allowed_restart_services":["docker.service","containerd.service","ssh.service"],
  "monitored_services":["docker.service","ssh.service"],
  "database_enabled":false
}
JSON
cp /root/payload/apply.pub /root/original-apply.pub
awk '{print $1, $2, "different-comment"}' /root/payload/scan.pub > /root/payload/apply.pub
if bash /fixture/install-host.sh /root/payload >/tmp/denied.out 2>&1; then
  printf 'identical gateway credential unexpectedly accepted\n' >&2; exit 1
fi
cp /root/original-apply.pub /root/payload/apply.pub
useradd --system --home-dir /var/lib/sifto-maintenance-ssh/scan sifto-maint-scan
if bash /fixture/install-host.sh /root/payload >/tmp/denied.out 2>&1; then
  printf 'unmanaged existing account unexpectedly accepted\n' >&2; exit 1
fi
userdel sifto-maint-scan
bash /fixture/install-host.sh /root/payload
for native in coolify_host.py coolify_plan.py; do
  cmp "/root/payload/maintenance/$native" "/opt/sifto-maintenance/maintenance/$native"
  [[ $(stat -c %u "/opt/sifto-maintenance/maintenance/$native") == 0 ]]
  [[ $(stat -c %a "/opt/sifto-maintenance/maintenance/$native") == 644 ]]
done
groupadd docker
usermod -a -G docker sifto-maint-scan
if bash /fixture/install-host.sh /root/payload >/tmp/denied.out 2>&1; then
  printf 'extra privileged group unexpectedly accepted\n' >&2; exit 1
fi
gpasswd -d sifto-maint-scan docker >/dev/null
printf 'sifto-maint-scan ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/extra-grant
chmod 0440 /etc/sudoers.d/extra-grant
if bash /fixture/install-host.sh /root/payload >/tmp/denied.out 2>&1; then
  printf 'extra sudo grant unexpectedly accepted\n' >&2; exit 1
fi
rm /etc/sudoers.d/extra-grant
bash /fixture/install-host.sh /root/payload
[[ $(stat -c %a /etc/sifto-maintenance/config.json) == 600 ]]
[[ $(stat -c %a /var/lib/sifto-maintenance/plans) == 700 ]]
[[ $(stat -c %u /var/lib/sifto-maintenance-ssh/scan/.ssh/authorized_keys) == 0 ]]
[[ $(stat -c %a /etc/sudoers.d/sifto-maintenance) == 440 ]]
if env SSH_ORIGINAL_COMMAND=report runuser -u sifto-maint-scan -- sudo -n /usr/bin/python3 -I -c 'print(1)' >/tmp/denied.out 2>&1; then
  printf 'unexpected unrestricted sudo command\n' >&2
  exit 1
fi
install -d -m 0755 /run/sshd
/usr/sbin/sshd -D -e -p 2222 >/tmp/sshd.log 2>&1 &
server=$!
trap 'kill "$server"; wait "$server" || true' EXIT
for attempt in $(seq 1 20); do
  if ssh-keyscan -T 1 -p 2222 localhost > /root/known_hosts 2>/dev/null; then break; fi
  sleep 1
done
ssh_call() {
  local role="$1" command="$2"
  ssh -T -F /dev/null -p 2222 -i "/root/payload/$role" \
    -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes \
    -o UserKnownHostsFile=/root/known_hosts -o ConnectTimeout=5 \
    "sifto-maint-$role@localhost" "$command"
}
report=$(printf '{}' | ssh_call scan report)
[[ "$report" == *'"host_id":"fixture"'* ]]
before=$(find /var/lib/sifto-maintenance -type f -printf '%p\n' | sort)
coolify_report=$(printf '{}' | ssh_call scan coolify-report)
[[ "$coolify_report" == *'"eligible":false'* && "$coolify_report" == *'"status":"disabled"'* ]]
after=$(find /var/lib/sifto-maintenance -type f -printf '%p\n' | sort)
[[ "$before" == "$after" ]]
if printf '{"ready":true}' | ssh_call scan coolify-report >/tmp/denied.out 2>&1; then
  printf 'caller-supplied Coolify readiness unexpectedly accepted\n' >&2; exit 1
fi
if printf '{}' | ssh_call apply coolify-report >/tmp/denied.out 2>&1; then
  printf 'apply key unexpectedly entered Coolify report gateway\n' >&2; exit 1
fi
for native in coolify_host.py coolify_plan.py; do
  cp -p "/opt/sifto-maintenance/maintenance/$native" /root/native.saved
  printf 'from pathlib import Path\nPath("/root/untrusted-native-executed").touch()\n' > "/opt/sifto-maintenance/maintenance/$native"
  chmod 0666 "/opt/sifto-maintenance/maintenance/$native"
  if printf '{}' | ssh_call scan coolify-report >/tmp/denied.out 2>&1; then
    printf 'unprotected native source unexpectedly accepted\n' >&2; exit 1
  fi
  [[ ! -e /root/untrusted-native-executed ]]
  [[ $(cat /tmp/denied.out) == *'maintenance bootstrap rejected'* ]]
  mv -f /root/native.saved "/opt/sifto-maintenance/maintenance/$native"
done
if printf '{}' | ssh_call scan apply >/tmp/denied.out 2>&1; then
  printf 'scan key unexpectedly entered apply gateway\n' >&2
  exit 1
fi
if printf '{}' | ssh_call apply report >/tmp/denied.out 2>&1; then
  printf 'apply key unexpectedly entered scan gateway\n' >&2
  exit 1
fi
before=$(find /var/lib/sifto-maintenance -type f -printf '%p\n' | sort)
if printf '{"dry_run":true}' | ssh_call apply apply >/tmp/denied.out 2>&1; then
  printf 'unsigned apply request unexpectedly accepted\n' >&2
  exit 1
fi
after=$(find /var/lib/sifto-maintenance -type f -printf '%p\n' | sort)
[[ "$before" == "$after" ]]
[[ -z $(find /opt/sifto-maintenance -name '*.pyc') ]]
# Installation refuses concurrent maintenance before replacing executable code.
flock /var/lib/sifto-maintenance/maintenance.lock sleep 10 &
holder=$!
sleep 0.2
if bash /fixture/install-host.sh /root/payload >/tmp/denied.out 2>&1; then
  printf 'concurrent install unexpectedly accepted\n' >&2
  exit 1
fi
kill "$holder"
wait "$holder" || true
printf 'real SSH forced commands, sudo limits and no-write dry-run verified\n'
