import hashlib
import importlib.util
import io
import unittest
import zipfile
from pathlib import Path

spec = importlib.util.spec_from_file_location('updater', Path(__file__).with_name('check-update.py'))
updater = importlib.util.module_from_spec(spec)
spec.loader.exec_module(updater)


class StableReleaseTest(unittest.TestCase):
    def test_select_stable_numeric_max_and_only_official_amalgamation(self):
        digest = 'a' * 64
        rows = [f'PRODUCT,{version},{path},100,{digest}' for version, path in [
            ('3.9.0', '2020/sqlite-amalgamation-3090000.zip'),
            ('3.54.1', '2026/sqlite-amalgamation-3540100.zip'),
            ('3.55.0-beta', '2026/sqlite-amalgamation-3550000.zip'),
            ('99.0.0', 'https://evil.example/sqlite-amalgamation-9990000.zip'),
            ('99.0.0', '2026/../sqlite-amalgamation-9990000.zip')]]
        self.assertEqual(updater.discover('\n'.join(rows))[1], '3.54.1')

    def test_reject_missing_release_and_invalid_checksum(self):
        for page in ['', 'PRODUCT,3.54.1,2026/sqlite-amalgamation-3540100.zip,100,invalid']:
            with self.assertRaises(ValueError):
                updater.discover(page)

    def test_validate_archive_digest_size_and_header_version(self):
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, 'w') as archive:
            archive.writestr('sqlite-amalgamation-3540100/sqlite3.h', '#define SQLITE_VERSION "3.54.1"')
        data = buffer.getvalue()
        release = ((3, 54, 1), '3.54.1', '2026/sqlite-amalgamation-3540100.zip', len(data), hashlib.sha3_256(data).hexdigest())
        self.assertEqual(updater.pin(release, data)['sha256'], hashlib.sha256(data).hexdigest())
        with self.assertRaises(ValueError):
            updater.pin(release, data + b'corruption')
        with self.assertRaises(ValueError):
            updater.pin((release[0], '3.54.2', *release[2:]), data)


if __name__ == '__main__':
    unittest.main()
