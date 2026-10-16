import { promises as fs } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const args = process.argv.slice(2);
const options = {};
for (let index = 0; index < args.length; index += 2) {
  const key = args[index];
  if (!['--vessel-root', '--plugin', '--arch', '--out'].includes(key) || !args[index + 1] || options[key]) {
    throw new Error('Usage: package-plugin.mjs --vessel-root <checkout> --plugin <name> [--arch amd64|arm64] --out <new-file.vesselhq>');
  }
  options[key] = args[index + 1];
}
const { '--vessel-root': vessel, '--plugin': plugin, '--out': output } = options;
const arch = options['--arch'] ?? 'amd64';
if (!vessel || !output || !['environment-summary', 'environment-summary-service', 'persistent-notes'].includes(plugin) || !['amd64', 'arm64'].includes(arch)) {
  throw new Error('A Vessel checkout, simple plugin name, output and supported Linux architecture are required');
}
const builder = path.resolve(vessel, 'scripts/native-extensions/builder.mjs');
await fs.access(builder);
try {
  await fs.lstat(path.resolve(output));
  throw new Error('Output already exists; choose a new destination');
} catch (error) {
  if (error.code !== 'ENOENT') throw error;
}
function run(command, parameters, extra = {}) {
  const result = spawnSync(command, parameters, { cwd: root, encoding: 'utf8', ...extra });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(result.stderr || result.stdout || `${command} failed`);
  return result.stdout;
}
const temporary = await fs.mkdtemp(path.join(tmpdir(), 'vessel-simple-plugin-'));
try {
  const staged = path.join(temporary, plugin);
  await fs.cp(path.join(root, 'plugins', plugin), staged, { recursive: true, dereference: false });
  const manifestFile = path.join(staged, 'manifest.json');
  const manifest = JSON.parse(await fs.readFile(manifestFile, 'utf8'));
  if (plugin === 'environment-summary-service') {
    await fs.cp(path.join(root, 'plugins/environment-summary/ui'), path.join(staged, 'ui'), { recursive: true });
    const executable = `backend/linux-${arch}/extension`;
    await fs.mkdir(path.dirname(path.join(staged, executable)), { recursive: true });
    run('go', ['build', '-mod=readonly', '-trimpath', '-ldflags=-s -w', '-o', path.join(staged, executable), './examples/environment-summary'], {
      env: { ...process.env, GOWORK: 'off', CGO_ENABLED: '0', GOOS: 'linux', GOARCH: arch },
    });
    manifest.backend.entrypoints = { [`linux-${arch}`]: executable };
    await fs.writeFile(manifestFile, JSON.stringify(manifest, null, 2) + '\n');
  }
  manifest.packageDigest = run(process.execPath, [builder, 'digest', staged]).trim();
  await fs.writeFile(manifestFile, JSON.stringify(manifest, null, 2) + '\n');
  process.stdout.write(run(process.execPath, [builder, 'pack', staged, path.resolve(output)]));
} finally {
  await fs.rm(temporary, { recursive: true, force: true });
}
