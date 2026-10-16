#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { constants, promises as fs } from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const roots = [
  'AGENTS.md', 'README.md', 'LICENSE', 'SDK_VERSION', 'go.mod', 'go.sum',
  'generate.sh', 'client.go', 'client_test.go', 'config.go', 'doc.go',
  'storage.go', 'proto', 'examples', 'plugins', 'ui', 'docs', 'scripts',
];
const suffixes = new Set(['.go', '.proto', '.md', '.ts', '.tsx', '.css', '.json', '.mjs']);
const names = new Set(['LICENSE', 'SDK_VERSION', 'go.mod', 'go.sum', 'generate.sh']);

/** Public source inventory; reject links even when their suffix is not allowed. */
export async function publicSources(root) {
  const sources = [];
  async function visit(relative) {
    const absolute = path.join(root, relative);
    let stat;
    try { stat = await fs.lstat(absolute); }
    catch (error) { if (error.code === 'ENOENT') return; throw error; }
    if (stat.isSymbolicLink()) throw new Error(`Symlinks are not distributable: ${relative}`);
    if (stat.isDirectory()) {
      for (const child of (await fs.readdir(absolute)).sort()) await visit(path.join(relative, child));
    } else if (stat.isFile() && !relative.split(path.sep).some((part) => part.startsWith('.')) &&
      (suffixes.has(path.extname(relative)) || names.has(path.basename(relative)))) {
      sources.push(relative);
    }
  }
  for (const relative of roots) await visit(relative);
  return sources.sort();
}

export async function main(args) {
  if (args.length !== 2 || args[0] !== '--out' || !args[1]) {
    throw new Error('Usage: node scripts/package-source.mjs --out <new-sdk.tar.gz>');
  }
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const output = path.resolve(args[1]);
  try {
    await fs.lstat(output);
    throw new Error('Output already exists; choose a new destination');
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  execFileSync('go', ['build', '-mod=readonly', './...'], {
    cwd: root, env: { ...process.env, GOWORK: 'off' }, stdio: 'inherit',
  });
  const sources = await publicSources(root);
  await fs.mkdir(path.dirname(output), { recursive: true });
  const temporary = await fs.mkdtemp(path.join(path.dirname(output), '.vessel-sdk-'));
  try {
    const staged = path.join(temporary, 'vesselhq-extension-sdk');
    await fs.mkdir(staged);
    for (const relative of sources) {
      const destination = path.join(staged, relative);
      await fs.mkdir(path.dirname(destination), { recursive: true });
      // Recheck the opened object: never follow a source link introduced after inventory.
      const handle = await fs.open(path.join(root, relative), constants.O_RDONLY | constants.O_NOFOLLOW);
      try {
        const stat = await handle.stat();
        if (!stat.isFile()) throw new Error(`Source is not a regular file: ${relative}`);
        await fs.writeFile(destination, await handle.readFile(), { flag: 'wx', mode: stat.mode & 0o777 });
      } finally { await handle.close(); }
    }
    const candidate = path.join(temporary, 'sdk.tar.gz');
    execFileSync('tar', ['-czf', candidate, '-C', temporary, 'vesselhq-extension-sdk'], { stdio: 'inherit' });
    // Same-filesystem hard link atomically refuses existing destinations, including races.
    await fs.link(candidate, output);
    const digest = createHash('sha256').update(await fs.readFile(output)).digest('hex');
    console.log(`${digest}  ${path.basename(output)}`);
  } finally {
    await fs.rm(temporary, { recursive: true, force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
