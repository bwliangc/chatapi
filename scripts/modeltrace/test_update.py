"""Offline end-to-end updater checks using a real temporary Git upstream."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class BuildUpdateTest(unittest.TestCase):
    def test_build_and_manual_update_and_algorithm_gate(self):
        with tempfile.TemporaryDirectory(prefix='modeltrace_build_test_') as temp:
            root = Path(temp) / 'app'
            root.mkdir()
            for directory in ['scripts/modeltrace', 'third_party/modeltrace', 'backend/internal/pkg/modeltrace']:
                shutil.copytree(ROOT / directory, root / directory)
            for name in ['go.mod', 'go.sum']:
                shutil.copy2(ROOT / 'backend' / name, root / 'backend' / name)
            upstream = Path(temp) / 'upstream'
            upstream.mkdir()
            def git(*args, cwd=upstream):
                return subprocess.check_output(['git', *args], cwd=cwd, stderr=subprocess.STDOUT).decode().strip()
            git('init', '-b', 'main')
            git('config', 'user.email', 'test@example.invalid')
            git('config', 'user.name', 'Updater Test')
            (upstream / 'static').mkdir()
            (upstream / 'data').mkdir()
            shutil.copy2(root / 'third_party/modeltrace/fingerprint-core.mjs', upstream / 'static/fingerprint-core.js')
            shutil.copy2(root / 'third_party/modeltrace/LICENSE', upstream / 'LICENSE')
            bank = json.loads((root / 'backend/internal/pkg/modeltrace/unified_bank.json').read_text())
            bank['built_at'] = '2030-01-01T00:00:00Z'
            (upstream / 'data/unified_bank.json').write_text(json.dumps(bank))
            git('add', '.')
            git('commit', '-m', 'new bank')
            sha = git('rev-parse', 'HEAD')
            # Fixed URL is rewritten only within the test process, so the real
            # fetch path works offline and never reaches GitHub.
            env = dict(os.environ, GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL='/dev/null',
                       GIT_CONFIG_COUNT='1', GIT_CONFIG_KEY_0=f'url.{upstream.as_uri()}.insteadOf',
                       GIT_CONFIG_VALUE_0='https://github.com/xqy2006/ModelTrace.git', GIT_ALLOW_PROTOCOL='file')
            def update(*args):
                return subprocess.run(['python3', str(root / 'scripts/modeltrace/update.py'), *args],
                                      cwd=root, env=env, capture_output=True, text=True)
            result = update('--build', '--apply')
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            source = json.loads((root / 'backend/internal/pkg/modeltrace/source.json').read_text())
            self.assertEqual(source['commit'], sha)
            self.assertEqual(source['bank_sha256'], hashlib.sha256((upstream / 'data/unified_bank.json').read_bytes()).hexdigest())
            self.assertFalse((root / '.git').exists())
            before = {p: p.read_bytes() for p in (root / 'backend/internal/pkg/modeltrace').rglob('*') if p.is_file()}
            (upstream / 'static/fingerprint-core.js').write_text('throw new Error("must never execute changed code");')
            git('add', '.')
            git('commit', '-m', 'incompatible algorithm')
            result = update('--build', '--apply')
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('algorithm changed', result.stderr)
            self.assertTrue(all(p.read_bytes() == data for p, data in before.items()))
            # The developer CLI also updates source.json/parity using local Git objects.
            git('init', cwd=root)
            git('config', 'user.email', 'test@example.invalid', cwd=root)
            git('config', 'user.name', 'Updater Test', cwd=root)
            git('add', '.', cwd=root)
            git('commit', '-m', 'baseline', cwd=root)
            git('fetch', str(upstream), sha, cwd=root)
            result = update('--no-fetch', '--ref', sha, '--apply')
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(git('status', '--porcelain', cwd=root), '')
            (root / 'backend/internal/pkg/modeltrace/core_test.go').unlink()
            result = update('--build', '--apply', '--no-fetch', '--ref', sha)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('validation tests are missing', result.stderr)


if __name__ == '__main__':
    unittest.main()
