"""Controller operations. All externally supplied values are data, never shell."""
import base64
import hashlib
import os
import sys
import time
import urllib.request
import uuid
from pathlib import Path

from .host import strict_json
from .plan import (HASH, SHA, MaintenanceError, build_plan, canonical_json, matches,
                   normalize_closure, plan_sha256, require, sign_plan, validate_approval, validate_plan)
from .report import ai_advice, render
from .ssh import Client, parse_hosts
from .store import GitStore, private_write, repo_url, validate_ref


def assess(inventory, executor, *, now=None):
    result = {"status": "unknown", "reason": "inventory_invalid", "update_count": None, "plan": None, "changes": []}
    try:
        require(type(inventory) is dict and type(inventory["simulation_available"]) is bool,
                "inventory_invalid")
        if not inventory["simulation_available"]:
            result["reason"] = "simulation_unavailable"
            return result
        result["changes"] = normalize_closure(inventory["simulation"], inventory["packages"])
        result["update_count"] = len(result["changes"])
        policy = inventory["maintenance_policy"]
        require(policy["host_role"] in ("application", "database") and type(policy["database_enabled"]) is bool
                and type(policy["backup_ready"]) is bool, "inventory_invalid")
        for blocked, reason in [(inventory["metadata"].get("fresh") is not True, "metadata_stale"),
                                (type(inventory["apt_locks"]) is not list or bool(inventory["apt_locks"]), "apt_locked"),
                                (inventory["dpkg_state"].get("ready") is not True, "dpkg_pending"),
                                (any(x != "active" for x in inventory["runtime"]["services"].values()), "service_unhealthy"),
                                (any(x["status"] != "running" or x["health"] not in ("healthy", "none")
                                     for x in inventory["runtime"]["containers"]), "container_unhealthy")]:
            if blocked:
                result.update(status="blocked", reason=reason)
                return result
        database_change = policy["host_role"] == "database" or any(
            x["name"].split(":")[0].startswith(("postgresql", "libpq", "pgbouncer", "pgpool")) for x in result["changes"])
        if database_change and not (policy["database_enabled"] and policy["backup_ready"]):
            result.update(status="blocked", reason="database_backup_or_enable_missing")
            return result
        if not result["changes"] and not inventory["reboot_required"]:
            result.update(status="current", reason="no_updates")
            return result
        reboot = not result["changes"] and inventory["reboot_required"] is True
        plan = build_plan(inventory, executor, now=now, action="reboot" if reboot else "apt_upgrade", allow_reboot=reboot)
        result.update(status="eligible", reason="human_approval_required", plan=plan)
        return result
    except (MaintenanceError, KeyError, TypeError, ValueError):
        result.update(status="blocked", reason="inventory_or_policy_invalid", plan=None)
        return result


def reserve_candidate(ledger, plan, *, force=False):
    stable = {key: value for key, value in plan.items() if key not in ("plan_id", "created_at", "expires_at")}
    fingerprint = hashlib.sha256(canonical_json(stable)).hexdigest()
    if fingerprint in ledger and not force:
        return False
    ledger[fingerprint] = {"plan_id": plan["plan_id"], "expires_at": plan["expires_at"]}
    return True


def execution_block(history, *, now=None):
    """Only a complete trusted root history can clear an interrupted execution.

    Root may append operator_diagnosed_at after diagnosis; the consumed journal
    remains present. A fresh plan still needs a fresh native human approval.
    """
    incomplete = {"status": "unknown", "reason": "execution_history_incomplete"}
    blocked = {"status": "blocked", "reason": "operator_diagnosis_required"}
    now = int(time.time()) if now is None else now
    if type(history) is not dict:
        return incomplete
    receipts = history.get("receipts")
    total, truncated = history.get("receipt_total"), history.get("receipt_truncated")
    if (type(receipts) is not list or len(receipts) > 20 or type(total) is not int
            or type(truncated) is not int or min(total, truncated) < 0 or total != len(receipts) + truncated):
        return incomplete
    for receipt in receipts:
        if type(receipt) is not dict or receipt.get("status") not in ("started", "failed", "completed", "reboot_scheduled", "verified"):
            return incomplete
        diagnosis = receipt.get("operator_diagnosed_at")
        started = receipt.get("failed_at", receipt.get("started_at", 0))
        diagnosed = type(diagnosis) is int and type(started) is int and 0 < diagnosis <= now and diagnosis >= started
        if receipt["status"] in ("started", "failed") and not diagnosed:
            return blocked
    if "receipt_history_complete" in history:
        count = history.get("unresolved_execution_count")
        if history["receipt_history_complete"] is not True or type(count) is not int or not 0 <= count <= total:
            return incomplete
        return blocked if count else None
    return incomplete if truncated else None


def actual_approver(history, allowlist):
    require(type(history) is list and type(allowlist) is list and bool(allowlist), "approval_history_missing")
    relevant = [entry for entry in history if type(entry) is dict and any(
        type(environment) is dict and environment.get("name") == "production-maintenance"
        for environment in entry.get("environments", []))]
    require(bool(relevant), "approval_history_missing")
    # Fail closed on any rejection/ambiguous reviewer, independent of REST
    # array ordering. A rerun with conflicting history needs operator review.
    require(all(entry.get("state") == "approved" and entry.get("user", {}).get("login") in allowlist
                for entry in relevant), "approved_reviewer_missing")
    entry = relevant[-1]
    login = entry.get("user", {}).get("login")
    require(entry.get("state") == "approved" and login in allowlist, "approved_reviewer_missing")
    return login


def checked_plan(plan, identifier, digest, executor, *, now=None):
    validate_plan(plan)
    require(plan["plan_id"] == identifier and matches(HASH, digest) and plan_sha256(plan) == digest,
            "immutable_plan_mismatch")
    require(matches(SHA, executor) and plan["workflow_sha"] == executor, "executor_pin_mismatch")
    now = int(time.time()) if now is None else now
    require(plan["created_at"] <= now < plan["expires_at"], "plan_expired")
    return plan


def github(path, *, method="GET", body=None):
    require(path.startswith("/repos/") and len(path) <= 512, "invalid GitHub API path")
    token = os.environ.get("GH_TOKEN", "")
    require(bool(token), "github_token_missing")
    request = urllib.request.Request("https://api.github.com" + path, method=method,
        data=canonical_json(body) if body is not None else None,
        headers={"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json",
                 "X-GitHub-Api-Version": "2022-11-28", "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=30) as response:
        raw = response.read(1024 * 1024 + 1)
    require(len(raw) <= 1024 * 1024, "github_response_limit")
    return strict_json(raw) if raw else None


def output(name, value):
    require(name in ("approval_commit", "approval_sha256"), "invalid public output")
    require(matches(SHA if name == "approval_commit" else HASH, value), "invalid output hash")
    path = os.environ.get("MAINTENANCE_OUTPUT", "/private/output")
    with open(path, "a", encoding="utf-8") as stream:
        stream.write(name + "=" + value + "\n")
    os.chmod(path, 0o600)


def summary(message):
    private_write(Path("/private/summary.md"), message.encode())


def configuration(operation):
    require(os.environ.get("GITHUB_REF") == "refs/heads/main", "main_only")
    executor = os.environ.get("MAINTENANCE_EXECUTOR_SHA", "")
    require(matches(SHA, executor), "invalid_executor_pin")
    require(os.environ.get("MAINTENANCE_CHECKED_OUT_SHA") == executor, "checkout_pin_mismatch")
    repo = os.environ.get("MAINTENANCE_OPS_REPO", "")
    repo_url(repo)
    inputs = strict_json(Path("/secrets/inputs.json").read_bytes())
    require(type(inputs) is dict and set(inputs) == {"private_commit", "plan_id", "plan_sha256", "executor_sha",
                                                     "approval_commit", "approval_sha256", "force_request", "dry_run"},
            "invalid_input_fields")
    require(type(inputs["force_request"]) is bool and type(inputs["dry_run"]) is bool, "invalid boolean inputs")
    if operation != "scan":
        validate_ref(inputs["private_commit"], "plans/" + inputs["plan_id"] + "/plan.json")
        try:
            require(str(uuid.UUID(inputs["plan_id"])) == inputs["plan_id"], "invalid_plan_id")
        except (ValueError, TypeError, AttributeError):
            raise MaintenanceError("invalid_plan_id") from None
        require(inputs["executor_sha"] == executor and matches(HASH, inputs["plan_sha256"]), "input_pin_mismatch")
    if operation == "execute":
        require(not Path("/secrets/approval_key").exists(), "executor_cannot_hold_signer")
        require(not Path("/secrets/ops_write_key").exists(), "executor_cannot_hold_writer")
    store = GitStore("/private/git", repo, Path("/secrets/ops_key"), Path("/secrets/known_hosts"),
                     writer=operation in ("scan", "authorize"))
    return executor, repo, inputs, store


def load_plan(store, inputs, executor):
    path = "plans/" + inputs["plan_id"] + "/plan.json"
    raw = store.fetch(inputs["private_commit"], path, digest=inputs["plan_sha256"])
    return checked_plan(strict_json(raw), inputs["plan_id"], inputs["plan_sha256"], executor)


def scan(executor, repo, inputs, store):
    hosts = parse_hosts(Path("/secrets/hosts.json").read_bytes())
    client = Client("/private", "/secrets/host_key", "/secrets/known_hosts", "scan")
    parent = store.head()
    ledger_raw = store.read_at(parent, "ledger.json", optional=True)
    ledger = strict_json(ledger_raw) if ledger_raw else {}
    require(type(ledger) is dict and all(matches(HASH, key) for key in ledger), "invalid_ledger")
    files, requests, counts = {}, [], {"eligible": 0, "blocked": 0, "unknown": 0, "current": 0}
    roles = {role: dict.fromkeys(counts, 0) for role in ("application", "database")}
    scan_id = str(uuid.uuid4())
    for host in hosts:
        prefix = "scans/" + scan_id + "/" + host["host_id"]
        result = {"status": "unknown", "reason": "collection_failed", "update_count": None, "plan": None, "changes": []}
        try:
            inventory = client.call(host, "scan")
            files[prefix + "/inventory.json"] = inventory
            require(inventory["host_id"] == host["host_id"], "host_identity_mismatch")
            require(inventory["maintenance_policy"]["host_role"] == host["role"], "host_role_mismatch")
            if inventory["metadata"].get("fresh") is not True:
                client.call(host, "refresh")
                inventory = client.call(host, "scan")
                require(inventory["host_id"] == host["host_id"]
                        and inventory["maintenance_policy"]["host_role"] == host["role"], "host_identity_mismatch")
                files[prefix + "/inventory.json"] = inventory
            history = client.call(host, "report")
            require(history.get("host_id") == host["host_id"], "receipt_identity_mismatch")
            files[prefix + "/root-receipts.json"] = history
            result = assess(inventory, executor)
            blocked = execution_block(history)
            if blocked:
                result.update(**blocked, plan=None)
                if blocked["status"] == "unknown":
                    result["update_count"] = None
        except Exception:
            # Transport diagnostics remain on the private filesystem only.
            result["reason"] = "collection_or_refresh_failed"
        counts[result["status"]] += 1
        roles[host["role"]][result["status"]] += 1
        plan = result["plan"]
        advice = ai_advice(plan, Path("/secrets/anthropic_key").read_text().strip(),
                           os.environ.get("MAINTENANCE_AI_MODEL") or "claude-haiku-4-5") if plan else {
                           "status": "unavailable", "warning_ja": "承認可能な計画がありません。AI 解析は実施していません。"}
        files[prefix + "/assessment.json"] = result
        files[prefix + "/report.md"] = render(result, advice).encode()
        if plan and reserve_candidate(ledger, plan, force=inputs["force_request"]):
            files["plans/" + plan["plan_id"] + "/plan.json"] = plan
            files["plans/" + plan["plan_id"] + "/report.md"] = files[prefix + "/report.md"]
            requests.append(plan)
    files["ledger.json"] = ledger
    # Raw transport diagnostics, including failed collection, stay private.
    for path in Path("/private/transport").glob("*/*.bin"):
        files["scans/" + scan_id + "/transport/" + path.parent.name + "/" + path.name] = path.read_bytes()
    files["audit/" + scan_id + ".json"] = {"operation": "scan", "executor_sha": executor,
                                             "pipeline_sha": os.environ.get("MAINTENANCE_PIPELINE_SHA"), "counts": counts}
    commit = store.write(parent, files)
    summary("Maintenance scan: " + str(counts) + "\nHost roles: " + str(roles) +
            "\n\nPrivate reports: https://github.com/" + repo + "/tree/" + commit + "/scans/" + scan_id + "\n" +
            "".join("\nPlan SHA256: " + plan_sha256(plan) + "\nPrivate report: https://github.com/" + repo +
                    "/blob/" + commit + "/plans/" + plan["plan_id"] + "/report.md\n" for plan in requests))
    for plan in requests:
        github("/repos/enjoydarts/sifto/actions/workflows/maintenance-apply.yml/dispatches", method="POST", body={
            "ref": "main", "inputs": {"private_commit": commit, "plan_id": plan["plan_id"],
                "plan_sha256": plan_sha256(plan), "executor_sha": executor,
                "dry_run": "true" if inputs["dry_run"] else "false"}})


def authorize(executor, repo, inputs, store):
    plan = load_plan(store, inputs, executor)
    run_id = os.environ.get("GITHUB_RUN_ID", "")
    import re
    require(re.fullmatch(r"[0-9]{1,30}", run_id) is not None, "invalid_run_id")
    allowlist = strict_json(os.environ.get("MAINTENANCE_ALLOWED_APPROVERS", "[]"))
    history = github("/repos/enjoydarts/sifto/actions/runs/" + run_id + "/approvals")
    reviewer = actual_approver(history, allowlist)
    now = int(time.time())
    approval = {"plan_sha256": plan_sha256(plan), "run_id": run_id, "approved_by": reviewer,
                "approved_at": now, "expires_at": min(plan["expires_at"], now + 3600), "workflow_sha": executor,
                "execution_mode": "dry_run" if inputs["dry_run"] else "apply"}
    key = base64.b64decode(Path("/secrets/approval_key").read_text().strip(), validate=True)
    require(len(key) == 32, "invalid_signer")
    envelope = sign_plan(plan, approval, key)
    path = "approvals/" + plan["plan_id"] + "/" + run_id + ".json"
    parent = store.head()
    commit = store.write(parent, {path: envelope, "audit/approval-" + run_id + ".json": {
        "operation": "authorize", "approved_by": reviewer, "executor_sha": executor,
        "pipeline_sha": os.environ.get("MAINTENANCE_PIPELINE_SHA"), "plan_commit": inputs["private_commit"],
        "plan_sha256": plan_sha256(plan), "dry_run": inputs["dry_run"]}})
    output("approval_commit", commit)
    output("approval_sha256", hashlib.sha256(canonical_json(envelope)).hexdigest())
    summary("Maintenance authorization recorded. Plan SHA256: " + plan_sha256(plan) + "\n")


def execute(executor, repo, inputs, store):
    plan = load_plan(store, inputs, executor)
    run_id = os.environ.get("GITHUB_RUN_ID", "")
    import re
    require(re.fullmatch(r"[0-9]{1,30}", run_id) is not None, "invalid_run_id")
    path = "approvals/" + inputs["plan_id"] + "/" + run_id + ".json"
    envelope = strict_json(store.fetch(inputs["approval_commit"], path, digest=inputs["approval_sha256"]))
    require(type(envelope) is dict and set(envelope) == {"plan", "approval", "signature"}
            and envelope["plan"] == plan, "signed_plan_mismatch")
    validate_approval(plan, envelope["approval"])
    require(envelope["approval"]["run_id"] == run_id, "approval_run_mismatch")
    require(envelope["approval"]["execution_mode"] == ("dry_run" if inputs["dry_run"] else "apply"), "signed_mode_mismatch")
    hosts = parse_hosts(Path("/secrets/hosts.json").read_bytes())
    host = next((host for host in hosts if host["host_id"] == plan["host_id"]), None)
    require(host is not None, "host_missing")
    client = Client("/private", "/secrets/host_key", "/secrets/known_hosts", "apply")
    # Exactly one apply attempt. Never resubmit even if SSH disconnects.
    result = client.call(host, "apply", {"workflow_sha": executor, "envelope": envelope, "dry_run": inputs["dry_run"]})
    if inputs["dry_run"]:
        require(result == {"plan_id": plan["plan_id"], "status": "preflight_passed"}, "preflight_invalid")
    elif plan["action"] == "reboot":
        require(result.get("status") == "reboot_scheduled", "apply_incomplete")
        deadline = time.monotonic() + 600
        while True:
            try:
                result = client.call(host, "verify", {"plan_id": plan["plan_id"]})
                break
            except MaintenanceError:
                require(time.monotonic() < deadline, "reboot_verify_unavailable")
                time.sleep(15)
    else:
        require(result.get("status") == "completed", "apply_incomplete")
        result = client.call(host, "verify", {"plan_id": plan["plan_id"]})
    require(result.get("plan_id") == plan["plan_id"] and result.get("status") in ("verified", "preflight_passed"),
            "verification_incomplete")
    summary("Maintenance result: " + result["status"] + ". Plan SHA256: " + plan_sha256(plan) + "\nRoot receipt will be audited by the next private scanner.\n")


def main():
    os.umask(0o077)
    try:
        require(len(sys.argv) == 2 and sys.argv[1] in ("scan", "prepare", "authorize", "execute"), "invalid_operation")
        operation = sys.argv[1]
        executor, repo, inputs, store = configuration(operation)
        if operation == "prepare":
            plan = load_plan(store, inputs, executor)
            summary("Approval requested. Execution mode: " + ("dry_run" if inputs["dry_run"] else "apply") +
                    ". Plan SHA256: " + plan_sha256(plan) + "\nPrivate report: https://github.com/" + repo +
                    "/blob/" + inputs["private_commit"] + "/plans/" + plan["plan_id"] + "/report.md\n")
        else:
            {"scan": scan, "authorize": authorize, "execute": execute}[operation](executor, repo, inputs, store)
        return 0
    except Exception:
        # Never interpolate an exception, argv, inventory or secret into CI logs.
        summary("Maintenance blocked: controller_operation_failed. Operator diagnosis required; no automatic apply retry.\n")
        print("maintenance_controller_failed", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
