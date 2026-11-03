#!/usr/bin/env python3
"""Install the pinned build-only compiler, verifying its official SHA-256."""
import hashlib
import os
from pathlib import Path
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
URL = 'https://ziglang.org/download/0.17.0/zig-x86_64-linux-0.17.0.tar.xz'
DIGEST = '1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026'
archive = ROOT/'.build/zig.tar.xz'
archive.parent.mkdir(parents=True,exist_ok=True)
if not archive.exists():
    with urllib.request.urlopen(URL,timeout=120) as response:
        archive.write_bytes(response.read())
if hashlib.sha256(archive.read_bytes()).hexdigest() != DIGEST:
    raise SystemExit('Zig compiler SHA-256 mismatch')
with tarfile.open(archive) as source:
    source.extractall(ROOT/'.build',filter='data')
path = str(ROOT/'.build/zig-x86_64-linux-0.17.0')
if os.environ.get('GITHUB_PATH'):
    with open(os.environ['GITHUB_PATH'],'a') as output:
        output.write(path+'\n')
print(path)
