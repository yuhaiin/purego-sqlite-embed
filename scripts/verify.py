#!/usr/bin/env python3
"""Verify every committed asset, manifest, and Go platform selection."""
import hashlib
import json
import os
import re
import struct
import subprocess
from pathlib import Path

from build import ROOT, SOURCE, FLAGS, BRIDGE_ABI, BRIDGE_SHA256


def verify():
    targets = json.loads((ROOT / 'scripts/targets.json').read_text())
    for key, target in targets.items():
        directory = ROOT / 'assets' / key
        manifest = json.loads((directory / 'manifest.json').read_text())
        filename = {'darwin': 'libsqlite3.dylib', 'windows': 'sqlite3.dll'}.get(target['goos'], 'libsqlite3.so')
        expected = {'sqlite_version': SOURCE['version'], 'source_url': SOURCE['url'],
                    'row_bridge_abi': BRIDGE_ABI, 'row_bridge_sha256': BRIDGE_SHA256,
                    'source_sha256': SOURCE['sha256'], 'source_sha3_256': SOURCE['sha3_256'],
                    'goos': target['goos'], 'goarch': target['goarch'], 'libc': target['libc'],
                    'float_abi': target.get('float_abi', ''), 'minimum_os': target['minimum_os'], 'filename': filename}
        for field, value in expected.items():
            if manifest.get(field) != value:
                raise ValueError(f'{key}: incorrect {field}')
        data = (directory / filename).read_bytes()
        if len(data) != manifest['size'] or hashlib.sha256(data).hexdigest() != manifest['sha256']:
            raise ValueError(f'{key}: incorrect binary size/hash')
        if not set(FLAGS).issubset(manifest['flags']):
            raise ValueError(f'{key}: required compile options missing')
        arch = target['goarch']
        if target['goos'] in ('linux', 'freebsd'):
            if data[:4] != b'\x7fELF' or data[5] != 1 or data[4] != (1 if arch == 'mipsle' else 2):
                raise ValueError(f'{key}: incorrect ELF format')
            if struct.unpack_from('<H', data, 18)[0] != {'amd64': 62, 'arm64': 183, 'mipsle': 8}[arch]:
                raise ValueError(f'{key}: incorrect ELF machine')
            if arch == 'mipsle':
                section_offset = struct.unpack_from('<I', data, 32)[0]
                section_size, section_count = struct.unpack_from('<HH', data, 46)
                abi = None
                for i in range(section_count):
                    section = section_offset + i * section_size
                    if struct.unpack_from('<I', data, section + 4)[0] == 0x7000002a:
                        offset = struct.unpack_from('<I', data, section + 16)[0]
                        abi = data[offset + 7]
                if abi != (1 if target['float_abi'] == 'hardfloat' else 3):
                    raise ValueError(f'{key}: incorrect MIPS FP ABI')
            if target['libc'] == 'glibc':
                versions = subprocess.check_output(['readelf', '--version-info', str(directory / filename)], text=True)
                if any(tuple(map(int, v.split('.'))) > (2, 17) for v in re.findall(r'GLIBC_([0-9.]+)', versions)):
                    raise ValueError(f'{key}: requires glibc newer than 2.17')
        elif target['goos'] == 'windows':
            offset = struct.unpack_from('<I', data, 60)[0]
            if data[:2] != b'MZ' or data[offset:offset+4] != b'PE\0\0' or struct.unpack_from('<H', data, offset+4)[0] != {'amd64': 0x8664, 'arm64': 0xaa64}[arch]:
                raise ValueError(f'{key}: incorrect PE machine')
        else:
            if data[:4] != b'\xcf\xfa\xed\xfe' or struct.unpack_from('<I', data, 4)[0] != {'amd64': 0x1000007, 'arm64': 0x100000c}[arch]:
                raise ValueError(f'{key}: incorrect Mach-O machine')
        env = os.environ | {'GOOS': target['goos'], 'GOARCH': arch, 'CGO_ENABLED': '0'}
        if arch == 'mipsle':
            env['GOMIPS'] = target['float_abi']
        tags = 'sqlite_musl' if target['libc'] == 'musl' else ''
        package = json.loads(subprocess.check_output(['go', 'list', '-json', '-tags', tags, '.'], env=env, cwd=ROOT))
        files = set(package.get('EmbedFiles', []))
        wanted = {f'assets/{key}/{filename}', f'assets/{key}/manifest.json'}
        if files != wanted:
            raise ValueError(f'{key}: wrong embedded target selection: {files}')
        print(f'verified {key}: {len(data)} bytes')
    for goos, arch in [('linux', '386'), ('freebsd', 'arm64'), ('android', 'arm64'), ('ios', 'arm64')]:
        package = json.loads(subprocess.check_output(['go', 'list', '-json', '.'], cwd=ROOT,
                             env=os.environ | {'GOOS': goos, 'GOARCH': arch, 'CGO_ENABLED': '0'}))
        if package.get('EmbedFiles'):
            raise ValueError(f'{goos}/{arch}: unsupported target embeds native code')


if __name__ == '__main__':
    verify()
