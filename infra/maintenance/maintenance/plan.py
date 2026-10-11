"""Canonical plans and Ed25519 approvals. This module never executes commands."""

import base64
import hashlib
import json
import re
import time
import uuid

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey


class MaintenanceError(ValueError):
    """A request cannot safely proceed; callers must not automatically retry."""


PACKAGE = re.compile(r"[a-z0-9][a-z0-9+.-]*(?::[a-z0-9][a-z0-9-]*)?\Z")
VERSION = re.compile(r"[0-9][A-Za-z0-9.+:~\-]*\Z")
SERVICE = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.@-]*\.service\Z")
SHA = re.compile(r"[0-9a-f]{40}\Z")
HASH = re.compile(r"[0-9a-f]{64}\Z")
HOST = re.compile(r"[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}\Z")
PLAN_FIELDS = {"schema_version", "plan_id", "host_id", "created_at", "expires_at",
               "workflow_sha", "state_sha256", "packages", "allow_service_restarts",
               "allow_reboot", "restart_services", "restart_scope", "expected_runtime"}
APPROVAL_FIELDS = {"plan_sha256", "run_id", "approved_by", "approved_at", "expires_at", "workflow_sha"}


def canonical_json(value):
    try:
        return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"),
                          allow_nan=False).encode("utf-8")
    except (TypeError, ValueError, UnicodeError) as exc:
        raise MaintenanceError("invalid canonical JSON") from exc


def plan_sha256(plan):
    return hashlib.sha256(canonical_json(plan)).hexdigest()


def require(condition, message):
    if not condition:
        raise MaintenanceError(message)


def matches(pattern, value):
    return isinstance(value, str) and pattern.fullmatch(value) is not None


def parse_simulation(output, *, canonical_architecture=False):
    """Read apt Inst and Conf records; configuration cannot escape Inst closure.

    Canonical identities always contain the target architecture, including :all.
    The default preserves apt's display names for callers presenting raw output.
    """
    require(isinstance(output, str), "simulation must be text")
    result = {}
    configurations = {}
    for line in output.splitlines():
        require(not line.startswith("Remv "), "package removal is forbidden")
        if not line.startswith(("Inst ", "Conf ")):
            continue
        match = re.fullmatch(r"(Inst|Conf) (\S+)(?: \[([^\]]+)\])? \((\S+) (.+)\)(?: \[.*\])?", line)
        require(match is not None, "malformed apt installation record")
        operation, name, installed, target, origin = match.groups()
        require(matches(PACKAGE, name) and matches(VERSION, target)
                and (installed is None or matches(VERSION, installed)), "invalid package or version")
        architecture = re.search(r"\[([a-z0-9][a-z0-9-]*)\]$", origin)
        require(architecture is not None, "apt target architecture is missing")
        base, separator, explicit_architecture = name.partition(":")
        architecture = architecture.group(1)
        require(not separator or explicit_architecture == architecture, "apt package architecture disagrees with target")
        identity = base + ":" + architecture
        if operation == "Conf":
            require(installed is None and identity not in configurations, "invalid or duplicate configure record")
            configurations[identity] = target
        else:
            require(identity not in result, "duplicate package in simulation")
            result[identity] = {"name": identity if canonical_architecture else name,
                                "installed": installed, "target": target}
    require(all(identity in result and result[identity]["target"] == target
                for identity, target in configurations.items()), "unapproved package configuration in simulation")
    return sorted(result.values(), key=lambda x: x["name"])


def normalize_closure(output, installed_packages):
    # Host inventories use Package:Architecture uniformly. Unqualified names are
    # accepted only for legacy native ARM64 inventory callers.
    installed = {(x["name"] if ":" in x["name"] else x["name"] + ":arm64"): x["version"] for x in installed_packages}
    require(len(installed) == len(installed_packages), "duplicate canonical installed package")
    result = parse_simulation(output, canonical_architecture=True)
    seen = set()
    for package in result:
        name = package["name"]
        require(name not in seen, "duplicate canonical package")
        seen.add(name)
        require(installed.get(name) == package["installed"], "simulation disagrees with installed state")
    return sorted(result, key=lambda x: x["name"])


def state_sha256(inventory):
    # Collection times, repository freshness, health checks and simulation are observations,
    # not stable identities. A boot, package, engine or container identity change is drift.
    runtime = inventory["runtime"]
    identities = {key: runtime[key] for key in ("boot_id", "running_kernel", "docker_version")}
    if "postgres_version_num" in runtime:
        identities["postgres_version_num"] = runtime["postgres_version_num"]
    if "containerd_version" in runtime:
        identities["containerd_version"] = runtime["containerd_version"]
    identities["containers"] = sorted(
        [{"id": x["id"], "image_id": x["image_id"], "image": x["image"]}
         for x in runtime["containers"]], key=lambda x: x["id"])
    return plan_sha256({"host_id": inventory["host_id"], "platform": inventory["platform"],
                        "packages": sorted(inventory["packages"], key=lambda x: x["name"]),
                        "runtime": identities})


def validate_plan(plan):
    require(type(plan) is dict and PLAN_FIELDS <= set(plan)
            and set(plan) <= PLAN_FIELDS | {"action"}, "invalid plan fields")
    require(type(plan["schema_version"]) is int and plan["schema_version"] == 1, "unsupported plan schema")
    try:
        require(str(uuid.UUID(plan["plan_id"])) == plan["plan_id"], "invalid plan ID")
    except (ValueError, TypeError, AttributeError) as exc:
        raise MaintenanceError("invalid plan ID") from exc
    require(matches(HOST, plan["host_id"]), "invalid host ID")
    require(matches(SHA, plan["workflow_sha"]) and matches(HASH, plan["state_sha256"]), "invalid state or workflow SHA")
    require(type(plan["created_at"]) is int and type(plan["expires_at"]) is int
            and 0 <= plan["created_at"] < plan["expires_at"] <= plan["created_at"] + 86400, "invalid plan lifetime")
    require(type(plan["allow_reboot"]) is bool and type(plan["allow_service_restarts"]) is bool, "invalid action flags")
    require(type(plan["restart_services"]) is list and len(plan["restart_services"]) <= 64
            and all(matches(SERVICE, s) for s in plan["restart_services"])
            and len(set(plan["restart_services"])) == len(plan["restart_services"]), "invalid restart services")
    require(not plan["restart_services"] or plan["allow_service_restarts"], "service restarts not approved")
    require(type(plan["packages"]) is list and len(plan["packages"]) <= 10000, "invalid packages")
    expected = plan["expected_runtime"]
    require(type(expected) is dict and {"running_kernel", "docker_version"} <= set(expected)
            and set(expected) <= {"running_kernel", "docker_version", "containerd_version", "postgres_version_num"}, "invalid expected runtime fields")
    require(isinstance(expected["running_kernel"], str) and re.fullmatch(r"[A-Za-z0-9_.+-]{1,128}", expected["running_kernel"])
            and isinstance(expected["docker_version"], str) and re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[+.-][A-Za-z0-9_.+-]+)?", expected["docker_version"]), "invalid expected runtime")
    require("postgres_version_num" not in expected or (isinstance(expected["postgres_version_num"], str)
            and re.fullmatch(r"[0-9]{6}", expected["postgres_version_num"])), "invalid expected PostgreSQL version")
    require("containerd_version" not in expected or (isinstance(expected["containerd_version"], str)
            and re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[+.-][A-Za-z0-9_.+-]+)?", expected["containerd_version"])),
            "invalid expected containerd version")
    names = set()
    for package in plan["packages"]:
        require(type(package) is dict and set(package) == {"name", "installed", "target"}, "invalid package fields")
        require(matches(PACKAGE, package["name"]) and matches(VERSION, package["target"])
                and (package["installed"] is None or matches(VERSION, package["installed"])), "invalid package or version")
        require(package["name"] not in names, "duplicate package")
        require(package["installed"] != package["target"], "unchanged package")
        names.add(package["name"])
    action = plan.get("action", "apt_upgrade")
    require(action in ("apt_upgrade", "reboot"), "invalid action")
    if action == "reboot":
        require(plan["allow_reboot"] and not plan["packages"] and not plan["restart_services"], "invalid reboot plan")
        require(plan["restart_scope"] == "none", "reboot cannot approve package-script restarts")
    else:
        require(plan["restart_scope"] == "package_scripts_and_listed_services", "apt maintainer-script restart scope must be explicit")
        require(not plan["packages"] or plan["allow_service_restarts"], "apt scripts require service restart approval")
        require(not plan["allow_reboot"], "reboot requires a separate explicit reboot plan")
    canonical_json(plan)
    return plan


def expected_runtime(inventory, packages, action, restart_services=()):
    runtime = inventory["runtime"]
    expected = {"running_kernel": runtime.get("latest_installed_kernel", runtime["running_kernel"])
                if action == "reboot" else runtime["running_kernel"], "docker_version": runtime["docker_version"]}
    if "postgres_version_num" in runtime:
        expected["postgres_version_num"] = runtime["postgres_version_num"]
    if "containerd_version" in runtime:
        expected["containerd_version"] = runtime["containerd_version"]
    native = inventory["platform"]["architecture"]
    effective = {}
    for package in inventory["packages"]:
        name, separator, architecture = package["name"].partition(":")
        if not separator or architecture in (native, "all"):
            require(name not in effective, "ambiguous native runtime package")
            effective[name] = package["version"]
    changed = set()
    for package in packages:
        name, separator, architecture = package["name"].partition(":")
        if not separator or architecture in (native, "all"):
            effective[name] = package["target"]
            changed.add(name)
    for key, names, unit in (("docker_version", {"docker-ce", "docker.io", "moby-engine"}, "docker.service"),
                             ("containerd_version", {"containerd", "containerd.io"}, "containerd.service")):
        if action == "reboot" or unit in restart_services or names & changed:
            candidates = [version for name, version in effective.items() if name in names]
            require(len(candidates) <= 1, "ambiguous daemon runtime package")
            if candidates:
                match = re.fullmatch(r"(?:\d+:)?(\d+\.\d+\.\d+)(?:[-~+].*)?", candidates[0])
                require(match is not None, "cannot derive daemon runtime target")
                expected[key] = match.group(1)
    if action == "reboot" or any(unit.startswith("postgresql") for unit in restart_services):
        major = int(runtime["postgres_version_num"]) // 10000 if "postgres_version_num" in runtime else None
        candidates = [version for name, version in effective.items()
                      if re.fullmatch(r"postgresql-\d+", name) and (major is None or name == "postgresql-" + str(major))]
        require(len(candidates) <= 1, "ambiguous PostgreSQL runtime package")
        if candidates:
            match = re.fullmatch(r"(?:\d+:)?(\d+)\.(\d+)(?:[-~+].*)?", candidates[0])
            require(match is not None and int(match.group(1)) >= 10, "cannot derive PostgreSQL runtime target")
            expected["postgres_version_num"] = str(int(match.group(1)) * 10000 + int(match.group(2)))
    return expected


def critical_restart_services(packages, policy):
    """Minimum explicit post-update restarts, separate from package-script consent.

    Apt maintainer scripts can restart other host services. This list constrains
    only the agent's explicit systemctl operations; it does not sandbox scripts.
    """
    require(type(policy) is dict, "invalid maintenance policy")
    allowed = policy.get("allowed_restart_services", [])
    database_units = policy.get("database_restart_services", [])
    for units in (allowed, database_units):
        require(type(units) is list and len(units) <= 64 and all(matches(SERVICE, x) for x in units),
                "invalid configured restart scope")
    names = {x["name"].split(":")[0] for x in packages}
    required = set()
    if names & {"docker-ce", "docker.io", "moby-engine", "containerd", "containerd.io"}:
        required.add("docker.service")
        if "containerd.service" in allowed:
            required.add("containerd.service")
    if names & {"containerd", "containerd.io"}:
        required.add("containerd.service")
    postgres_change = "postgresql-common" in names or any(re.match(r"postgresql-\d+(?:-|$)", name) for name in names)
    library_change = policy.get("host_role") == "database" and any(
        name.startswith("lib") or name in ("openssl", "zlib1g") for name in names)
    if postgres_change or library_change:
        require(database_units and all(re.fullmatch(r"postgresql(?:@[A-Za-z0-9_.-]+)?\.service", unit) for unit in database_units),
                "native database restart units must be configured explicitly")
        required.update(database_units)
    for package, unit in (("pgbouncer", "pgbouncer.service"), ("pgpool2", "pgpool2.service")):
        if package in names:
            required.add(unit)
    require(required <= set(allowed), "critical service restart is not in root whitelist")
    return sorted(required)


def build_plan(inventory, workflow_sha, allow_service_restarts=True, allow_reboot=False, *, now=None, action="apt_upgrade"):
    now = int(time.time()) if now is None else now
    require(inventory["platform"] == {"id": "ubuntu", "version": "24.04", "architecture": "arm64"}, "unsupported platform")
    dpkg_state = inventory.get("dpkg_state", {"ready": True})
    require(type(dpkg_state) is dict and dpkg_state.get("ready") is True, "pending dpkg configuration requires operator recovery")
    require(action != "apt_upgrade" or inventory.get("simulation_available", True) is True,
            "update availability is unknown; no upgrade plan can be built")
    packages = normalize_closure(inventory["simulation"], inventory["packages"]) if action == "apt_upgrade" else []
    restart_services = critical_restart_services(packages, inventory.get("maintenance_policy", {})) if action == "apt_upgrade" else []
    plan = {"schema_version": 1, "plan_id": str(uuid.uuid4()), "host_id": inventory["host_id"],
            "created_at": now, "expires_at": now + 86400, "workflow_sha": workflow_sha,
            "state_sha256": state_sha256(inventory),
            "packages": packages, "expected_runtime": expected_runtime(inventory, packages, action, restart_services),
            "allow_service_restarts": allow_service_restarts, "allow_reboot": allow_reboot,
            "restart_services": restart_services,
            "restart_scope": "package_scripts_and_listed_services" if action == "apt_upgrade" else "none", "action": action}
    return validate_plan(plan)


def validate_approval(plan, approval):
    require(type(approval) is dict and set(approval) == APPROVAL_FIELDS, "invalid approval fields")
    require(approval["plan_sha256"] == plan_sha256(plan), "plan hash mismatch")
    require(approval["workflow_sha"] == plan["workflow_sha"], "approval workflow mismatch")
    require(isinstance(approval["run_id"], str) and re.fullmatch(r"[0-9]{1,30}", approval["run_id"]) is not None, "invalid run ID")
    require(isinstance(approval["approved_by"], str) and re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]{0,99}", approval["approved_by"]) is not None, "invalid approver")
    require(type(approval["approved_at"]) is int and type(approval["expires_at"]) is int
            and plan["created_at"] <= approval["approved_at"] < approval["expires_at"] <= plan["expires_at"], "invalid approval lifetime")


def sign_plan(plan, approval, private_key):
    validate_plan(plan)
    validate_approval(plan, approval)
    if isinstance(private_key, bytes):
        private_key = Ed25519PrivateKey.from_private_bytes(private_key)
    body = {"plan": plan, "approval": approval}
    return {**body, "signature": base64.b64encode(private_key.sign(canonical_json(body))).decode("ascii")}


def verify_envelope(envelope, public_key, host_id, workflow_sha, *, now=None):
    now = int(time.time()) if now is None else now
    require(type(envelope) is dict and set(envelope) == {"plan", "approval", "signature"}, "invalid envelope")
    plan, approval = envelope["plan"], envelope["approval"]
    validate_plan(plan)
    validate_approval(plan, approval)
    require(plan["host_id"] == host_id, "host mismatch")
    require(plan["workflow_sha"] == workflow_sha, "workflow mismatch")
    require(plan["created_at"] <= now < min(plan["expires_at"], approval["expires_at"])
            and approval["approved_at"] <= now, "approval expired or not yet valid")
    try:
        if isinstance(public_key, str):
            public_key = base64.b64decode(public_key, validate=True)
        if isinstance(public_key, bytes):
            public_key = Ed25519PublicKey.from_public_bytes(public_key)
        signature = base64.b64decode(envelope["signature"], validate=True)
        public_key.verify(signature, canonical_json({"plan": plan, "approval": approval}))
    except (InvalidSignature, ValueError, TypeError) as exc:
        raise MaintenanceError("invalid approval signature") from exc
    return plan
