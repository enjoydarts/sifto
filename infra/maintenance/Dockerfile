FROM python:3.12-slim
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends openssh-client git && rm -rf /var/lib/apt/lists/*
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
# OpenSSH requires a passwd entry for the caller UID used at runtime.
ARG MAINTENANCE_CONTROLLER_UID=0
ARG MAINTENANCE_CONTROLLER_GID=0
RUN python - "$MAINTENANCE_CONTROLLER_UID" "$MAINTENANCE_CONTROLLER_GID" <<'PY'
import grp, pwd, sys
from pathlib import Path
assert all(value.isascii() and value.isdecimal() for value in sys.argv[1:])
uid, gid = map(int, sys.argv[1:])
assert 0 <= uid < 2**31 and 0 <= gid < 2**31
try:
    grp.getgrgid(gid)
except KeyError:
    with Path('/etc/group').open('a') as stream:
        stream.write(f'maintenance{gid}:x:{gid}:\n')
try:
    pwd.getpwuid(uid)
except KeyError:
    with Path('/etc/passwd').open('a') as stream:
        stream.write(f'maintenance{uid}:x:{uid}:{gid}:Maintenance controller:/tmp:/usr/sbin/nologin\n')
PY
COPY maintenance ./maintenance
COPY tests ./tests
COPY playbooks ./playbooks
ENV PYTHONDONTWRITEBYTECODE=1
CMD ["python", "-B", "-m", "unittest", "discover", "-s", "tests", "-v"]
