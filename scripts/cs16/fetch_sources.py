import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
import urllib.error
import urllib.request
import zipfile


ROOT = Path(__file__).resolve().parents[2]
OWNED = ROOT / ".local/cs16"
TEMP = OWNED / "tmp"
RESEARCH = OWNED / "research"
METADATA = ROOT / "games/cs16/assets"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, _request, _file, _code, _message, _headers, _url):
        if _file is not None:
            _file.close()
        raise ValueError("Source redirects are forbidden")


def relative_parts(value):
    if not isinstance(value, str) or not value or any(character in value for character in ("\\", "\0", ":")):
        raise ValueError("Invalid source path")
    parts = value.split("/")
    if any(part in ("", ".", "..") for part in parts) or PurePosixPath(value).is_absolute():
        raise ValueError("Source path escapes its directory")
    return parts


def safe_components(path):
    path = Path(path).absolute()
    try:
        parts = path.relative_to(ROOT).parts
    except ValueError as error:
        raise ValueError("Path must remain inside the workspace") from error
    current = ROOT
    for part in parts:
        if part in (".", ".."):
            raise ValueError("Unsafe workspace path")
        current = current / part
        if current.is_symlink():
            raise ValueError("Symbolic links are forbidden: " + str(current.relative_to(ROOT)))
    return path


def owned_path(path):
    path = safe_components(path)
    if path != OWNED and OWNED not in path.parents:
        raise ValueError("Output must remain in .local/cs16")
    return path


def make_directory(path):
    path = owned_path(path)
    current = ROOT
    for part in path.relative_to(ROOT).parts:
        current = current / part
        if current.is_symlink():
            raise ValueError("Symbolic output directory")
        if current.exists():
            if not current.is_dir():
                raise ValueError("Output component is not a directory")
        else:
            current.mkdir()
    return path


def checked_target(directory, relative):
    directory = owned_path(directory)
    target = safe_components(directory.joinpath(*relative_parts(relative)))
    if target.exists() and not target.is_file():
        raise ValueError("Output target is not a regular file")
    return target


def entry_valid(entry):
    relative_parts(entry.get("path"))
    size = entry.get("bytes")
    if not isinstance(size, int) or isinstance(size, bool) or size <= 0 or size > 128 * 1024 * 1024:
        raise ValueError("Invalid source size")
    if not isinstance(entry.get("sha256"), str) or not re.fullmatch(r"[0-9a-f]{64}", entry["sha256"]):
        raise ValueError("Invalid source SHA-256")
    if "git_blob_sha1" in entry and not re.fullmatch(r"[0-9a-f]{40}", str(entry["git_blob_sha1"])):
        raise ValueError("Invalid Git blob digest")


def validate_manifest(manifest, kind):
    if not isinstance(manifest, dict) or not isinstance(manifest.get("files"), list) or not manifest["files"]:
        raise ValueError("Invalid source manifest")
    if kind == "community":
        sources = manifest.get("sources")
        if manifest.get("schema") != 1 or not isinstance(sources, dict) or not sources or len(sources) > 32:
            raise ValueError("Invalid community source directory")
        for name, source in sources.items():
            if not re.fullmatch(r"[a-z][a-z0-9-]*", name) or not isinstance(source, dict) or not re.fullmatch(r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", str(source.get("repository"))) or not re.fullmatch(r"[0-9a-f]{40}", str(source.get("commit"))):
                raise ValueError("Community source repository or ref is not pinned")
        seen = set()
        for entry in manifest["files"]:
            if not isinstance(entry, dict):
                raise ValueError("Invalid community source entry")
            entry_valid(entry)
            path = entry["path"]
            if path.casefold() in seen or not (path.startswith("original/") or path.startswith("navigation/")):
                raise ValueError("Duplicate or unsupported community source path")
            seen.add(path.casefold())
            source = sources.get(entry.get("source"))
            if source is None:
                raise ValueError("Unknown community source repository")
            source_path = entry.get("source_path")
            relative_parts(source_path)
            expected = "https://raw.githubusercontent.com/" + source["repository"].removeprefix("https://github.com/") + "/" + source["commit"] + "/" + source_path
            if entry.get("url") != expected or not entry.get("git_blob_sha1"):
                raise ValueError("Unpinned community source URL or Git blob")
        return manifest["files"]
    if kind == "original":
        repository = "https://github.com/martinsam16/cstrike"
        ref = manifest.get("commit")
        prefix = "https://raw.githubusercontent.com/martinsam16/cstrike/"
    elif kind == "navigation":
        repository = "https://github.com/hpoon/HLDS-CS1.6"
        ref = manifest.get("ref")
        prefix = "https://raw.githubusercontent.com/hpoon/HLDS-CS1.6/"
    else:
        raise ValueError("Unsupported source group")
    if manifest.get("repository") != repository or not isinstance(ref, str) or not re.fullmatch(r"[0-9a-f]{40}", ref):
        raise ValueError("Source repository or commit is not pinned")
    seen = set()
    for entry in manifest["files"]:
        if not isinstance(entry, dict):
            raise ValueError("Invalid source entry")
        entry_valid(entry)
        path = entry["path"]
        if path.casefold() in seen:
            raise ValueError("Duplicate source path")
        seen.add(path.casefold())
        if kind == "original":
            expected = prefix + ref + "/opt/hlds/" + path
            if entry.get("source_path") != "opt/hlds/" + path or entry.get("url") != expected:
                raise ValueError("Unpinned original source URL")
        else:
            if not path.startswith("navigation/") or len(relative_parts(path)) != 2 or not path.endswith(".pwf"):
                raise ValueError("Invalid navigation source path")
            expected = prefix + ref + "/cstrike/addons/podbot/wptdefault/" + PurePosixPath(path).name
            if entry.get("source") != expected:
                raise ValueError("Unpinned navigation source URL")
    return manifest["files"]


def verify_file(path, entry):
    path = safe_components(path)
    if not path.exists():
        return False
    if not path.is_file() or path.stat().st_size != entry["bytes"]:
        return False
    sha256 = hashlib.sha256()
    git_blob = hashlib.sha1(b"blob " + str(entry["bytes"]).encode() + b"\0")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, "rb") as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError("Source cache is not a regular file")
        while data := source.read(1024 * 1024):
            sha256.update(data)
            git_blob.update(data)
    return sha256.hexdigest() == entry["sha256"] and ("git_blob_sha1" not in entry or git_blob.hexdigest() == entry["git_blob_sha1"])


def fetch_entry(entry, directory, check=False):
    entry_valid(entry)
    target = checked_target(directory, entry["path"])
    if verify_file(target, entry):
        return {"path": entry["path"], "bytes": entry["bytes"], "sha256": entry["sha256"], "status": "reused"}
    if check:
        raise ValueError("Missing or invalid cached source: " + entry["path"])
    source_url = entry.get("url", entry.get("source"))
    if not isinstance(source_url, str):
        raise ValueError("Missing source URL")
    make_directory(target.parent)
    make_directory(TEMP)
    descriptor, temporary_name = tempfile.mkstemp(prefix="cs16-source-", dir=TEMP)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as output:
            request = urllib.request.Request(source_url, headers={"User-Agent": "wire-board-cs16-source-fetch"})
            opener = urllib.request.build_opener(NoRedirect())
            try:
                with opener.open(request, timeout=30) as response:
                    size = response.headers.get("Content-Length")
                    if size is not None and int(size) != entry["bytes"]:
                        raise ValueError("Source length mismatch: " + entry["path"])
                    received = 0
                    while data := response.read(min(1024 * 1024, entry["bytes"] - received + 1)):
                        received += len(data)
                        if received > entry["bytes"]:
                            raise ValueError("Source exceeds declared size: " + entry["path"])
                        output.write(data)
            except (OSError, urllib.error.URLError) as error:
                if isinstance(error, urllib.error.HTTPError):
                    error.close()
                raise ValueError("Source request failed: " + entry["path"]) from error
            output.flush()
            os.fsync(output.fileno())
        if not verify_file(temporary, entry):
            raise ValueError("Source digest or size mismatch: " + entry["path"])
        safe_components(target)
        os.replace(temporary, target)
        return {"path": entry["path"], "bytes": entry["bytes"], "sha256": entry["sha256"], "status": "downloaded"}
    finally:
        if temporary.exists():
            temporary.unlink()


def write_metadata(target, value):
    target = owned_path(target)
    data = (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()
    if target.is_file() and target.read_bytes() == data:
        return
    make_directory(target.parent)
    make_directory(TEMP)
    descriptor, temporary_name = tempfile.mkstemp(prefix="cs16-source-", dir=TEMP)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        safe_components(target)
        os.replace(temporary, target)
    finally:
        if temporary.exists():
            temporary.unlink()


def extract_reference(archive, directory, check=False):
    archive = owned_path(archive)
    directory = owned_path(directory)
    written = 0
    reused = 0
    with zipfile.ZipFile(archive) as bundle:
        members = []
        roots = set()
        seen = set()
        total = 0
        for item in bundle.infolist():
            parts = relative_parts(item.filename.rstrip("/"))
            roots.add(parts[0])
            mode = item.external_attr >> 16
            if stat.S_ISLNK(mode) or stat.S_IFMT(mode) not in (0, stat.S_IFREG, stat.S_IFDIR):
                raise ValueError("Unsupported reference archive member")
            if item.filename.casefold() in seen:
                raise ValueError("Duplicate reference archive member")
            seen.add(item.filename.casefold())
            total += item.file_size
            if len(seen) > 15000 or total > 256 * 1024 * 1024:
                raise ValueError("Reference archive exceeds its budget")
            if item.is_dir():
                continue
            if len(parts) < 2:
                raise ValueError("Reference archive has no root directory")
            target = checked_target(directory, "/".join(parts[1:]))
            members.append((item, target))
        if len(roots) != 1:
            raise ValueError("Reference archive has multiple roots")
        for item, target in members:
            data = bundle.read(item)
            entry = {"path": "/".join(target.relative_to(directory).parts), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
            if verify_file(target, entry):
                reused += 1
                continue
            if check:
                raise ValueError("Reference source member missing or invalid: " + entry["path"])
            make_directory(target.parent)
            make_directory(TEMP)
            descriptor, temporary_name = tempfile.mkstemp(prefix="cs16-source-", dir=TEMP)
            temporary = Path(temporary_name)
            try:
                with os.fdopen(descriptor, "wb") as output:
                    output.write(data)
                    output.flush()
                    os.fsync(output.fileno())
                safe_components(target)
                os.replace(temporary, target)
                written += 1
            finally:
                if temporary.exists():
                    temporary.unlink()
    return {"written": written, "reused": reused}


def load_metadata(name):
    path = safe_components(METADATA / name)
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        raise ValueError("Source metadata is not an object")
    return value


def fetch_sources(check=False, reference=False):
    original = load_metadata("original-sources.json")
    navigation = load_metadata("navigation-sources.json")
    original_entries = validate_manifest(original, "original")
    navigation_entries = validate_manifest(navigation, "navigation")
    jobs = [(entry, RESEARCH / "original") for entry in original_entries] + [(entry, RESEARCH) for entry in navigation_entries]
    community = None
    if (METADATA / "community-sources.json").is_file():
        community = load_metadata("community-sources.json")
        jobs.extend((entry, RESEARCH) for entry in validate_manifest(community, "community"))
    records = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        futures = [pool.submit(fetch_entry, entry, directory, check) for entry, directory in jobs]
        for future in futures:
            records.append(future.result())
    extracted = None
    if reference:
        source = load_metadata("reference-source.json")
        commit = source.get("commit")
        expected = "https://codeload.github.com/rehlds/ReGameDLL_CS/zip/" + str(commit)
        if source.get("repository") != "https://github.com/rehlds/ReGameDLL_CS" or not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit) or source.get("url") != expected or source.get("archive") != ".local/cs16/research/regamedll-reference.zip":
            raise ValueError("Reference source is not pinned")
        entry = {"path": "regamedll-reference.zip", "url": source["url"], "bytes": source["bytes"], "sha256": source["sha256"]}
        records.append(fetch_entry(entry, RESEARCH, check))
        extracted = extract_reference(RESEARCH / "regamedll-reference.zip", RESEARCH / "regamedll-reference", check)
        if not check:
            write_metadata(RESEARCH / "reference-source.json", source)
    if not check:
        write_metadata(RESEARCH / "original-samples.json", original)
        write_metadata(RESEARCH / "navigation-samples.json", navigation)
        if community is not None:
            write_metadata(RESEARCH / "community-source-catalog.json", community)
    return {"passed": True, "check": check, "reference": reference, "files": len(records), "reused": sum(row["status"] == "reused" for row in records), "downloaded": sum(row["status"] == "downloaded" for row in records), "verifiedBytes": sum(row["bytes"] for row in records), "referenceMembers": extracted}


def main():
    parser = argparse.ArgumentParser(description="Explicit local CS 1.6 fixed-source recovery; never run automatically in CI")
    parser.add_argument("--check", action="store_true", help="Only verify local files; never access the network or repair files")
    parser.add_argument("--reference", action="store_true", help="Also recover the pinned ReGameDLL reference source archive")
    options = parser.parse_args()
    print(json.dumps(fetch_sources(options.check, options.reference), ensure_ascii=False, sort_keys=True))


if __name__ == "__main__":
    main()
