#!/usr/bin/env python3
"""Check SQLite's official stable release metadata and pin verified source."""
import argparse
import csv
import hashlib
import io
import json
import os
from pathlib import Path
import re
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
DOWNLOAD = "https://www.sqlite.org/download.html"


def discover(page):
    # SQLite publishes machine-readable PRODUCT rows inside the download page.
    releases = []
    for line in page.splitlines():
        if not line.startswith("PRODUCT,"):
            continue
        fields = next(csv.reader([line]))
        if len(fields) != 5:
            continue
        _, version, path, size, digest = fields
        if not re.fullmatch(r"\d+\.\d+\.\d+", version):
            continue
        if not re.fullmatch(r"\d{4}/sqlite-amalgamation-\d+\.zip", path):
            continue
        if not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise ValueError("invalid official SHA3-256")
        releases.append((tuple(map(int,version.split('.'))), version, path, int(size), digest))
    if not releases:
        raise ValueError("no stable amalgamation in official metadata")
    return max(releases)


def pin(release, data):
    _, version, path, size, digest = release
    if len(data) != size or hashlib.sha3_256(data).hexdigest() != digest:
        raise ValueError("downloaded official source size/SHA3-256 mismatch")
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        header = archive.read(next(n for n in archive.namelist() if n.endswith('/sqlite3.h'))).decode()
    if not re.search(r'#define\s+SQLITE_VERSION\s+"' + re.escape(version) + r'"', header):
        raise ValueError("source header does not match stable release version")
    return {"version": version, "url": "https://www.sqlite.org/" + path,
            "sha256": hashlib.sha256(data).hexdigest(), "sha3_256": digest}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--force', action='store_true', help='rebuild the pinned release even without an update')
    args = parser.parse_args()
    current = json.loads((ROOT/'scripts/source.json').read_text())
    with urllib.request.urlopen(DOWNLOAD, timeout=60) as response:
        latest = discover(response.read().decode())
    old = tuple(map(int,current['version'].split('.')))
    if latest[0] < old:
        raise SystemExit('official metadata is older than the pin; refusing downgrade')
    changed = latest[0] > old
    if changed:
        with urllib.request.urlopen('https://www.sqlite.org/' + latest[2],timeout=60) as response:
            updated = pin(latest,response.read())
        (ROOT/'scripts/source.json').write_text(json.dumps(updated,indent=2)+'\n')
    outputs = {'changed':str(changed or args.force).lower(), 'version':latest[1] if changed else current['version']}
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'],'a') as stream:
            for key,value in outputs.items():
                stream.write(f'{key}={value}\n')
    print(json.dumps(outputs))


if __name__ == '__main__':
    main()
