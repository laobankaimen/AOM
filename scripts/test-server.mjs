import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import {randomBytes} from 'node:crypto';
mkdirSync('.cache', { recursive: true });
// A supplied binary lets the same browser suite verify the shipped program.
const prebuilt = process.env.AOM_E2E_BINARY;
const executable = prebuilt ? path.resolve(prebuilt) : path.resolve('.cache', process.platform === 'win32' ? 'aom-e2e.exe' : 'aom-e2e');
const env = { ...process.env, GOCACHE: process.env.GOCACHE || path.join(tmpdir(), 'aom-go-cache'), CGO_ENABLED: '0' };
if (!prebuilt) {
 const build = spawnSync('go', ['build', '-o', executable, './cmd/aom'], { stdio: 'inherit', env });
 if (build.status !== 0) process.exit(build.status || 1);
}
const data = mkdtempSync(path.join(tmpdir(), 'aom-e2e-'));
const setupToken=randomBytes(32).toString('base64url');
writeFileSync('.cache/e2e-setup-token',setupToken,{mode:0o600});
const child = spawn(executable, ['--addr', '127.0.0.1:18080', '--data-dir', data], { stdio: 'inherit',env:{...process.env,AOM_SETUP_TOKEN:setupToken} });
const stop = () => child.kill('SIGINT');
process.on('SIGINT', stop);process.on('SIGTERM', stop);
child.on('exit', code => { rmSync(data, { recursive: true, force: true }); rmSync('.cache/e2e-setup-token',{force:true}); process.exit(code || 0); });
