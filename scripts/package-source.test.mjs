import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync, spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { publicSources } from './package-source.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const script = path.join(root, 'scripts/package-source.mjs');
async function temporary(t) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'vessel-sdk-package-test-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  return directory;
}

test('allowlist excludes local state and credentials and rejects source symlinks', async (t) => {
  const directory = await temporary(t);
  for (const relative of ['README.md', 'ui/index.tsx', 'ui/identity.key', 'ui/.env', '.idea/private.xml', 'dist/sdk.tar.gz', '.env']) {
    await fs.mkdir(path.dirname(path.join(directory, relative)), { recursive: true });
    await fs.writeFile(path.join(directory, relative), 'fixture');
  }
  assert.deepEqual(await publicSources(directory), ['README.md', 'ui/index.tsx']);
  await fs.symlink(path.join(directory, 'README.md'), path.join(directory, 'ui/linked.key'));
  await assert.rejects(publicSources(directory), /Symlinks are not distributable/);
});

test('CLI refuses an existing output without changing it', async (t) => {
  const output = path.join(await temporary(t), 'preserved.tar.gz');
  await fs.writeFile(output, 'keep');
  const result = spawnSync(process.execPath, [script, '--out', output], { encoding: 'utf8' });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /already exists/);
  assert.equal(await fs.readFile(output, 'utf8'), 'keep');
});

test('public archive has expected layout and independently builds after extraction', async (t) => {
  const directory = await temporary(t);
  const output = path.join(directory, 'sdk.tar.gz');
  const result = execFileSync(process.execPath, [script, '--out', output], { encoding: 'utf8', cwd: root });
  const digest = createHash('sha256').update(await fs.readFile(output)).digest('hex');
  assert.equal(result.trim(), `${digest}  sdk.tar.gz`);
  const entries = execFileSync('tar', ['-tzf', output], { encoding: 'utf8' }).trim().split('\n');
  assert.ok(entries.includes('vesselhq-extension-sdk/go.mod'));
  assert.ok(entries.includes('vesselhq-extension-sdk/proto/v1/extension.proto'));
  assert.ok(entries.includes('vesselhq-extension-sdk/plugins/persistent-notes/manifest.json'));
  assert.ok(entries.every((name) => name.startsWith('vesselhq-extension-sdk/')));
  assert.ok(entries.every((name) => !/(?:^|\/)(?:\.idea|\.git|\.env|dist|node_modules|stack-network-overview)(?:\/|$)|\.(?:key|crt|pem|py)$/.test(name)));
  const extracted = path.join(directory, 'extracted');
  await fs.mkdir(extracted);
  execFileSync('tar', ['-xzf', output, '-C', extracted]);
  execFileSync('go', ['build', '-mod=readonly', './...'], {
    cwd: path.join(extracted, 'vesselhq-extension-sdk'), env: { ...process.env, GOWORK: 'off' }, stdio: 'pipe',
  });
});
