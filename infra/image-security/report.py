"""Correlate an exported final image, source revision, SBOM and findings."""
import hashlib
import json
import os
from pathlib import Path
import tarfile

EXPECTED = {
    "api": ("linux/arm64", "ca-certificates", "github.com/jackc/pgx/v5"),
    "worker": ("linux/arm64", "libc6", "fastapi"),
    "audio-concat": ("linux/amd64", "ffmpeg", "boto3"),
    "maintenance": ("linux/amd64", "openssh-client", "ansible-core"),
    "web": ("linux/amd64", "musl", "next"),
}


def digest(path):
    with path.open("rb") as stream:
        return "sha256:" + hashlib.file_digest(stream, "sha256").hexdigest()


def archive_identity(path):
    # Read members directly; never extract untrusted paths onto the filesystem.
    with tarfile.open(path) as archive:
        manifest = json.load(archive.extractfile("manifest.json"))
        if len(manifest) != 1:
            raise ValueError("Expected one final image in the Docker archive")
        raw = archive.extractfile(manifest[0]["Config"]).read()
    config = json.loads(raw)
    image_id = "sha256:" + hashlib.sha256(raw).hexdigest()
    return image_id, config["os"] + "/" + config["architecture"], config


def read_json(path):
    return json.loads(path.read_text())


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def prepare(directory, component):
    platform, _, _ = EXPECTED[component]
    image_id, actual_platform, config = archive_identity(directory / "image.tar")
    source_sha = (directory / "source-sha.txt").read_text().strip()
    if len(source_sha) != 40 or any(c not in "0123456789abcdef" for c in source_sha):
        raise ValueError("Source revision must be a full commit SHA")
    if actual_platform != platform or (directory / "platform.txt").read_text().strip() != platform:
        raise ValueError("Archive platform does not match the production platform")
    if config.get("config", {}).get("Labels", {}).get("org.opencontainers.image.revision") != source_sha:
        raise ValueError("Archive revision label does not match source commit")
    metadata = {
        "component": component,
        "source_commit": source_sha,
        "source_tree_dirty": (directory / "source-dirty.txt").read_text().strip() == "true",
        "platform": actual_platform,
        "dockerfile": (directory / "dockerfile.txt").read_text().strip(),
        "stage": "final",
        "image_id": image_id,
        "archive_sha256": digest(directory / "image.tar"),
        "build_image_id": (directory / "image-id.txt").read_text().strip(),
        "trivy_version": "0.75.0",
        "coverage": ("Production Dockerfile image; Vercel managed runtime OS is not inspected"
                     if component == "web" else "Production Dockerfile image for deployment platform"),
    }
    write_json(directory / "metadata.json", metadata)
    return metadata


def verify(directory, component):
    metadata = read_json(directory / "metadata.json")
    image_id, platform, config = archive_identity(directory / "image.tar")
    if metadata["component"] != component or metadata["image_id"] != image_id:
        raise ValueError("Metadata and scanned archive image IDs differ")
    if metadata["platform"] != platform or metadata["archive_sha256"] != digest(directory / "image.tar"):
        raise ValueError("Archive changed after metadata generation")
    if config.get("config", {}).get("Labels", {}).get("org.opencontainers.image.revision") != metadata["source_commit"]:
        raise ValueError("Metadata source commit differs from the image revision label")
    sbom = read_json(directory / "sbom.cdx.json")
    report = read_json(directory / "vulnerabilities.json")
    if sbom.get("bomFormat") != "CycloneDX":
        raise ValueError("Expected a CycloneDX SBOM")
    properties = {p["name"]: p["value"] for p in sbom.get("metadata", {}).get("component", {}).get("properties", [])}
    if properties.get("aquasecurity:trivy:ImageID") != image_id:
        raise ValueError("CycloneDX SBOM refers to a different image")
    if report.get("Metadata", {}).get("ImageID") != image_id:
        raise ValueError("Vulnerability findings refer to a different image")
    results = report.get("Results", [])
    os_names = {p["Name"] for r in results if r.get("Class") == "os-pkgs" for p in r.get("Packages", [])}
    library_names = {p["Name"] for r in results if r.get("Class") == "lang-pkgs" for p in r.get("Packages", [])}
    _, expected_os, expected_library = EXPECTED[component]
    sbom_names = {p["name"] for p in sbom.get("components", [])}
    # Known installed fixtures prove both inventories are populated, even with zero CVEs.
    for name, names in ((expected_os, os_names), (expected_library, library_names)):
        if name not in names or name not in sbom_names:
            raise ValueError(f"Missing known installed OS/library package: {name}")
    counts = {severity: 0 for severity in ("UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL")}
    for result in results:
        for vulnerability in result.get("Vulnerabilities", []):
            counts[vulnerability["Severity"]] += 1
    status = read_json(directory / "scan-status.json")
    expected_status = 1 if sum(counts.values()) else 0
    if status["sbom_exit_code"] != 0 or status["vulnerability_exit_code"] != expected_status:
        raise ValueError("Scanner exit status does not match the generated reports")
    metadata["report_sha256"] = {name: digest(directory / name) for name in ("sbom.cdx.json", "vulnerabilities.json", "scan-status.json")}
    metadata["package_counts"] = {"os": len(os_names), "library": len(library_names)}
    metadata["vulnerability_counts"] = counts
    write_json(directory / "metadata.json", metadata)
    summary = "\n".join([
        f"### {component} ({platform})", metadata["coverage"],
        f"Commit: `{metadata['source_commit']}`; local changes: `{metadata['source_tree_dirty']}`",
        f"Final image ID: `{image_id}`", f"Archive digest: `{metadata['archive_sha256']}`",
        f"Packages: OS {len(os_names)}, library {len(library_names)}.",
        "Vulnerabilities: " + ", ".join(f"{key} {value}" for key, value in counts.items()) + ".",
        "All severities and unfixed CVEs are reported; findings fail this advisory job.",
        "CycloneDX SBOM, JSON findings and correlation metadata are attached as artifacts.", "",
    ])
    (directory / "summary.md").write_text(summary)
    print(summary)
    return counts


if __name__ == "__main__":
    import sys
    directory = Path("/reports")
    component = os.environ["IMAGE_COMPONENT"]
    if sys.argv[1] == "prepare":
        prepare(directory, component)
    elif sys.argv[1] == "verify":
        verify(directory, component)
    else:
        raise SystemExit("Unknown report command")
