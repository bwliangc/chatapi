#!/usr/bin/env python3
"""Fetch a pinned ModelTrace snapshot; validate the Go port before applying it.

No upstream Python server, dependencies, or deployment code are installed.
Only the reference JS scorer is executed locally for parity validation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
UPSTREAM = 'https://github.com/xqy2006/ModelTrace.git'
FILES = {
    'static/fingerprint-core.js': 'third_party/modeltrace/fingerprint-core.mjs',
    'data/unified_bank.json': 'backend/internal/pkg/modeltrace/unified_bank.json',
    'LICENSE': 'third_party/modeltrace/LICENSE',
}
TRACKED = list(FILES.values()) + ['third_party/modeltrace/upstream.json', 'backend/internal/pkg/modeltrace/testdata/parity.json', 'backend/internal/pkg/modeltrace/source.json']


def run(*args, cwd=ROOT):
    return subprocess.check_output(args, cwd=cwd)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ref', default='main', help='upstream branch, tag or commit; default main')
    parser.add_argument('--apply', action='store_true', help='validate and apply snapshots (default: inspect only)')
    parser.add_argument('--no-fetch', action='store_true', help='resolve --ref from existing Git objects without network')
    parser.add_argument('--build', action='store_true', help='automatic build update: fixed upstream, unchanged algorithm only, no working-tree Git required')
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._/-]*', args.ref):
        raise SystemExit('Invalid ref')
    if args.build and not args.apply:
        raise SystemExit('--build requires --apply')
    if args.apply and any(not (ROOT / 'backend/internal/pkg/modeltrace' / name).is_file()
                          for name in ('core_test.go', 'bank_test.go')):
        raise SystemExit('ModelTrace validation tests are missing from the build context; update refused.')
    if args.apply and not args.build and run('git', 'status', '--porcelain', '--', *TRACKED).strip():
        raise SystemExit('Snapshot files have local changes. Commit or preserve them before updating.')
    if args.no_fetch:
        commit = run('git', 'rev-parse', '--verify', args.ref + '^{commit}').decode().strip()
        snapshots = {dest: run('git', 'show', f'{commit}:{src}') for src, dest in FILES.items()}
    elif args.build:
        # Docker contexts do not include .git. Use a disposable repository and
        # never alter the application's remotes or source checkout.
        with tempfile.TemporaryDirectory(prefix='modeltrace_fetch_') as temp:
            subprocess.run(['git', 'init', '--bare', temp], check=True, stdout=subprocess.DEVNULL)
            subprocess.run(['git', '-C', temp, 'fetch', '--depth=1', '--no-tags', UPSTREAM, args.ref], check=True)
            commit = run('git', 'rev-parse', 'FETCH_HEAD^{commit}', cwd=temp).decode().strip()
            snapshots = {dest: run('git', 'show', f'{commit}:{src}', cwd=temp) for src, dest in FILES.items()}
    else:
        url = run('git', 'config', '--get', 'remote.modeltrace.url').decode().strip()
        if url.removesuffix('.git') not in (UPSTREAM.removesuffix('.git'), 'git@github.com:xqy2006/ModelTrace'):
            raise SystemExit('The modeltrace remote must point to ' + UPSTREAM)
        subprocess.run(['git', 'fetch', '--no-tags', 'modeltrace', args.ref], cwd=ROOT, check=True)
        commit = run('git', 'rev-parse', 'FETCH_HEAD^{commit}').decode().strip()
        snapshots = {dest: run('git', 'show', f'{commit}:{src}') for src, dest in FILES.items()}
    record = {'repository': UPSTREAM, 'commit': commit, 'files': [
        {'source': src, 'local': dest, 'sha256': hashlib.sha256(snapshots[dest]).hexdigest()}
        for src, dest in FILES.items()
    ]}
    metadata = {'commit': commit, 'algorithm_sha256': record['files'][0]['sha256'], 'bank_sha256': record['files'][1]['sha256']}
    if args.build:
        compatible = json.loads((ROOT / 'backend/internal/pkg/modeltrace/source.json').read_text())
        if metadata['algorithm_sha256'] != compatible['algorithm_sha256']:
            raise SystemExit('ModelTrace algorithm changed. Upgrade/review the Go port before building; no snapshots were modified.')
    current = json.loads((ROOT / 'third_party/modeltrace/upstream.json').read_text())
    changed = [dest for dest, data in snapshots.items() if (ROOT / dest).read_bytes() != data]
    print('Current:', current['commit'])
    print('Candidate:', commit)
    print('Changed snapshot files:', ', '.join(changed) or '(none)')
    if not args.apply:
        print('Inspection only. Review upstream changes, then rerun with --apply and the exact candidate SHA.')
        return
    # The temporary package shares the backend module so it uses the existing Go
    # dependency/toolchain policy. Live sources remain untouched on test failure.
    package = ROOT / 'backend/internal/pkg/modeltrace'
    with tempfile.TemporaryDirectory(prefix='modeltrace_update_', dir=package.parent) as temp:
        stage = Path(temp)
        shutil.copytree(package, stage, dirs_exist_ok=True)
        (stage / 'source.json').write_text(json.dumps(metadata, indent=2) + '\n')
        (stage / 'unified_bank.json').write_bytes(snapshots[FILES['data/unified_bank.json']])
        (stage / 'fingerprint-core.mjs').write_bytes(snapshots[FILES['static/fingerprint-core.js']])
        subprocess.run(['node', str(ROOT / 'scripts/modeltrace/generate-parity.mjs'), str(stage / 'fingerprint-core.mjs'), str(stage / 'unified_bank.json'), str(stage / 'testdata/parity.json')], check=True, cwd=ROOT)
        subprocess.run(['go', 'test', './internal/pkg/' + stage.name, '-count=1'], cwd=ROOT / 'backend', check=True)
        snapshots['backend/internal/pkg/modeltrace/testdata/parity.json'] = (stage / 'testdata/parity.json').read_bytes()
        snapshots['backend/internal/pkg/modeltrace/source.json'] = (stage / 'source.json').read_bytes()
        snapshots['third_party/modeltrace/upstream.json'] = (json.dumps(record, indent=2) + '\n').encode()
        for dest, data in snapshots.items():
            # Atomic replacement per file; a failed validation never reaches here.
            target = ROOT / dest
            with tempfile.NamedTemporaryFile(dir=target.parent, delete=False) as tmp:
                tmp.write(data)
                temp_name = tmp.name
            os.chmod(temp_name, 0o644)
            os.replace(temp_name, target)
    print('Snapshot updated after parity validation. Review git diff, run project checks, then commit.')


if __name__ == '__main__':
    main()
