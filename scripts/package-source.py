#!/usr/bin/env python3
"""Build and package public SDK workspace sources without local or runtime state."""
import argparse
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    output = args.out.resolve()
    if output.exists() or output.is_symlink():
        parser.error('output already exists; choose a new destination')
    subprocess.run(['go', 'build', '-mod=readonly', './...'], cwd=root,
                   env={**os.environ, 'GOWORK': 'off'}, check=True)
    roots = ['AGENTS.md', 'README.md', 'LICENSE', 'SDK_VERSION', 'go.mod', 'go.sum',
             'generate.sh', 'client.go', 'client_test.go', 'config.go', 'doc.go',
             'storage.go', 'proto', 'examples', 'plugins', 'ui', 'docs', 'scripts']
    sources = []
    allowed = {'.go', '.proto', '.md', '.ts', '.tsx', '.css', '.json', '.mjs', '.py'}
    names = {'LICENSE', 'SDK_VERSION', 'go.mod', 'go.sum', 'generate.sh'}
    for relative in roots:
        entry = root / relative
        candidates = entry.rglob('*') if entry.is_dir() else [entry]
        for source in candidates:
            if source.is_symlink():
                raise SystemExit('Symlinks are not distributable: ' + str(source))
            if not source.is_file():
                continue
            if any(part.startswith('.') for part in source.relative_to(root).parts):
                continue
            if source.suffix in allowed or source.name in names:
                sources.append(source)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='vessel-sdk-', dir=output.parent) as temp:
        candidate = Path(temp) / 'sdk.tar.gz'
        with tarfile.open(candidate, 'w:gz') as archive:
            for source in sorted(sources):
                archive.add(source, arcname=Path('vesselhq-extension-sdk') / source.relative_to(root), recursive=False)
        os.link(candidate, output)
    print(hashlib.sha256(output.read_bytes()).hexdigest() + '  ' + output.name)

if __name__ == '__main__':
    main()
