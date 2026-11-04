#!/usr/bin/env python3
"""Build a pinned SQLite candidate; no executable binaries are downloaded."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shlex
import subprocess
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
SOURCE = json.loads((ROOT / "scripts/source.json").read_text())
VERSION, URL, SHA256, SHA3 = (SOURCE[key] for key in ("version", "url", "sha256", "sha3_256"))
FLAGS = ["-DSQLITE_THREADSAFE=1", "-DSQLITE_ENABLE_FTS5", "-DSQLITE_ENABLE_RTREE",
         "-DSQLITE_ENABLE_DBSTAT_VTAB", "-DSQLITE_ENABLE_MATH_FUNCTIONS", "-DSQLITE_USE_URI=1"]
BRIDGE = ROOT / "bridge/row.c"
BRIDGE_ABI = 1
BRIDGE_SHA256 = hashlib.sha256(BRIDGE.read_bytes()).hexdigest()


def source():
    cache = ROOT / ".build/source"
    cache.mkdir(parents=True, exist_ok=True)
    archive = cache / ("sqlite-" + VERSION + ".zip")
    if not archive.exists():
        with urllib.request.urlopen(URL, timeout=60) as response:
            data = response.read()
        if hashlib.sha256(data).hexdigest() != SHA256 or hashlib.sha3_256(data).hexdigest() != SHA3:
            raise SystemExit("official SQLite source checksum mismatch")
        archive.write_bytes(data)
    data = archive.read_bytes()
    if hashlib.sha256(data).hexdigest() != SHA256 or hashlib.sha3_256(data).hexdigest() != SHA3:
        raise SystemExit("cached source checksum mismatch")
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        for name in ("sqlite3.c", "sqlite3.h", "sqlite3ext.h"):
            (cache / name).write_bytes(archive.read(next(n for n in archive.namelist() if n.endswith("/" + name))))
    return cache


def build(key):
    targets = json.loads((ROOT / "scripts/targets.json").read_text())
    target = targets[key]
    src = source()
    out = ROOT / ".build/dist" / key
    out.mkdir(parents=True, exist_ok=True)
    goos = target["goos"]
    filename = {"darwin":"libsqlite3.dylib", "windows":"sqlite3.dll"}.get(goos, "libsqlite3.so")
    compiler = ["zig", "cc", "-target", target["zig"], "-mcpu=" + target.get("cpu", "baseline")] if "zig" in target else shlex.split(os.environ.get("CC", "clang" if goos == "windows" else "cc"))
    if "zig" in target and subprocess.check_output(["zig", "version"], text=True).strip() != "0.17.0":
        raise SystemExit("use pinned Zig 0.17.0")
    args = ["-O2", "-g0", "-dynamiclib" if goos == "darwin" else "-shared"]
    if goos != "windows":
        args += ["-fPIC"]
    if goos == "darwin":
        args += ["-mmacosx-version-min=11.0", "-Wl,-install_name,@rpath/libsqlite3.dylib"]
    elif goos == "linux":
        args += ["-Wl,-soname,libsqlite3.so", "-Wl,--build-id=sha1", "-Wl,-s"]
    if goos in ("linux", "freebsd"):
        args += ["-Wl,-Bsymbolic"]
    if goos == "windows":
        args += ["-DSQLITE_API=__declspec(dllexport)"]
    args += FLAGS
    libs = ["-lm", "-lpthread", "-ldl"] if goos == "linux" else ([] if goos == "windows" else ["-lm", "-lpthread"])
    command = compiler + args + ["-I" + str(src), str(src / "sqlite3.c"), str(BRIDGE), "-o", str(out / filename)] + libs
    print(shlex.join(command), flush=True)
    subprocess.run(command, check=True)
    data = (out / filename).read_bytes()
    manifest = {"sqlite_version": VERSION, "row_bridge_abi": BRIDGE_ABI, "row_bridge_sha256": BRIDGE_SHA256,
                "goos": goos, "goarch": target["goarch"],
                "libc": target["libc"], "float_abi": target.get("float_abi", ""),
                "minimum_os": target["minimum_os"], "filename": filename,
                "sha256": hashlib.sha256(data).hexdigest(), "size": len(data),
                "source_url": URL, "source_sha256": SHA256, "source_sha3_256": SHA3,
                "compiler": ("Zig 0.17.0; " if "zig" in target else "") + subprocess.check_output(compiler + ["--version"], text=True).splitlines()[0],
                "flags": (compiler[2:] if "zig" in target else []) + args + libs, "build_commit": os.environ.get("GITHUB_SHA") or subprocess.check_output(["git","rev-parse","HEAD"],cwd=ROOT,text=True).strip(),
                "build_run": os.environ.get("GITHUB_SERVER_URL", "") + "/" + os.environ.get("GITHUB_REPOSITORY", "") + "/actions/runs/" + os.environ.get("GITHUB_RUN_ID", "") if os.environ.get("GITHUB_RUN_ID") else "local"}
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("targets", nargs="+")
    for key in p.parse_args().targets:
        build(key)
