// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Starts the app the way a user gets it: the EXE with --data <copy of the test data> --no-browser,
// on a free port, offline. Used by the Playwright fixtures and by start.mjs.
import { spawn, spawnSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { delimiter, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
export const ROOT = join(here, '..');
export const TESTDATA = join(here, 'testdata');
const win = process.platform === 'win32';
// in the data directory: written by a test, it lists the microphones the app sees ("* " marks the default)
export const MIKROFONE = 'mikrofone.txt';

// The EXE under test: TRANSCRIBE_EXE if set (e.g. a release build), else built from this checkout.
export function exePath() {
  return process.env.TRANSCRIBE_EXE || join(here, '.bin', win ? 'transcribe.exe' : 'transcribe');
}

export function buildExe() {
  if (process.env.TRANSCRIBE_EXE) {
    if (!existsSync(process.env.TRANSCRIBE_EXE)) throw new Error(`TRANSCRIBE_EXE nicht gefunden: ${process.env.TRANSCRIBE_EXE}`);
    return process.env.TRANSCRIBE_EXE;
  }
  const out = exePath();
  const r = spawnSync('go', ['build', '-o', out, './cmd/transcribe'], {
    cwd: ROOT, stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0' },
  });
  if (r.status !== 0) throw new Error('go build ./cmd/transcribe ist fehlgeschlagen');
  return out;
}

// The stand-in for speech recognition services (internal/sttapi/sttapitest), built next to the EXE.
const fakePath = join(here, '.bin', win ? 'fake-api.exe' : 'fake-api');

export function buildFake() {
  const out = fakePath;
  const r = spawnSync('go', ['build', '-o', out, './e2e/fake-api'], {
    cwd: ROOT, stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0' },
  });
  if (r.status !== 0) throw new Error('go build ./e2e/fake-api ist fehlgeschlagen');
  return out;
}

// Starts the stand-in; returns { url, stop }. Its OpenAI-compatible API is at <url>/v1, ElevenLabs' at <url>.
export async function startFakeAPI() {
  if (!existsSync(fakePath)) buildFake();
  const child = spawn(fakePath, [], { windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
  const url = await new Promise((resolve, reject) => {
    let output = '';
    const timer = setTimeout(() => reject(new Error('API-Nachbau hat sich nicht gemeldet:\n' + output)), 20000);
    child.stdout.setEncoding('utf8').on('data', chunk => {
      output += chunk;
      const m = /http:\/\/127\.0\.0\.1:\d+/.exec(output);
      if (m) { clearTimeout(timer); resolve(m[0]); }
    });
    child.once('exit', code => { clearTimeout(timer); reject(new Error(`API-Nachbau beendet (${code}):\n${output}`)); });
  });
  return { url, stop: () => { if (child.exitCode === null) child.kill(); } };
}

function onPath(tool) {
  return spawnSync(win ? 'where' : 'which', [tool], { stdio: 'ignore' }).status === 0;
}

// The app's environment with the folder of ffmpeg first on its PATH: the app looks ffmpeg up at start,
// and searching a long PATH takes seconds on some machines, longer than a test waits.
function withToolsFirst(env) {
  const r = spawnSync(win ? 'where' : 'which', ['ffmpeg'], { encoding: 'utf8' });
  const first = r.status === 0 ? r.stdout.split(/\r?\n/)[0].trim() : '';
  if (!first) return env;
  // on Windows the variable is usually spelled Path; a second one beside it would be ambiguous
  const key = Object.keys(env).find(k => k.toUpperCase() === 'PATH') ?? 'PATH';
  return { ...env, [key]: dirname(first) + delimiter + (env[key] ?? '') };
}

// What the first run would download. Empty files are enough: the app only checks that they exist,
// and nothing in the tests runs a recognition that would load them.
const PLACEHOLDERS = {
  runtime: [join('lib', win ? 'sherpa-onnx-c-api.dll' : process.platform === 'darwin' ? 'libsherpa-onnx-c-api.dylib' : 'libsherpa-onnx-c-api.so')],
  silero: ['models/silero_vad.onnx'],
  parakeet: ['models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/encoder.int8.onnx'],
  german: ['models/sherpa-onnx-nemo-transducer-stt_de_fastconformer_hybrid_large_pc-int8/encoder.int8.onnx'],
  'whisper-base': ['models/sherpa-onnx-whisper-base/base-encoder.int8.onnx'],
  segmentation: ['models/sherpa-onnx-pyannote-segmentation-3-0/model.onnx'],
  campplus: ['models/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx'],
  ced: ['models/sherpa-onnx-ced-small-audio-tagging-2024-04-19/model.int8.onnx'],
};

function freePort() {
  return new Promise((resolve, reject) => {
    const s = createServer();
    s.unref();
    s.on('error', reject);
    s.listen(0, '127.0.0.1', () => { const { port } = s.address(); s.close(() => resolve(port)); });
  });
}

/**
 * Starts the EXE on a fresh copy of the test data.
 * missing: names from PLACEHOLDERS to leave out (their download then fails, the app is offline).
 * api: start the stand-in for speech recognition services too; its address is apiUrl.
 * env: more environment variables for the app.
 * Returns { url, dataDir, apiUrl, stop }.
 */
export async function startApp({ missing = [], exe = exePath(), api = false, env = {} } = {}) {
  if (!existsSync(exe)) throw new Error(`EXE fehlt: ${exe} (erst buildExe() oder TRANSCRIBE_EXE setzen)`);
  if (!onPath('ffmpeg') || !onPath('ffprobe')) {
    throw new Error('ffmpeg und ffprobe müssen im PATH liegen (Wellenform, Clip-Export); sonst lädt die App sie bei jedem Start herunter.');
  }
  const dataDir = mkdtempSync(join(tmpdir(), 'tx-e2e-'));
  cpSync(TESTDATA, dataDir, { recursive: true });
  for (const [name, files] of Object.entries(PLACEHOLDERS)) {
    if (missing.includes(name)) continue;
    for (const f of files) { mkdirSync(dirname(join(dataDir, f)), { recursive: true }); writeFileSync(join(dataDir, f), ''); }
  }

  const fake = api ? await startFakeAPI() : null;
  const port = await freePort();
  const child = spawn(exe, ['--data', dataDir, '--no-browser', '--port', String(port), '--threads', '2'], {
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
    // offline: every download fails at once instead of fetching gigabytes; SHERPA_LIB would point elsewhere.
    // MIKROFONE, once written, stands in for the microphones connected (see internal/live/devices.go).
    env: {
      ...withToolsFirst(process.env), HTTP_PROXY: 'http://127.0.0.1:9', HTTPS_PROXY: 'http://127.0.0.1:9', NO_PROXY: '', SHERPA_LIB: '', TRANSCRIBE_DATA: '',
      TRANSCRIBE_LIVE_DEVICES: join(dataDir, MIKROFONE), ...env,
    },
  });
  let output = '';
  const exited = new Promise(resolve => child.once('exit', resolve));
  const stop = async () => {
    fake?.stop();
    if (child.exitCode === null) {
      // with the ffmpeg it may have started (waveform, clip export), which hold files in the data directory
      if (win) spawnSync('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
      else child.kill();
      await exited;
    }
    try { rmSync(dataDir, { recursive: true, force: true, maxRetries: 20, retryDelay: 150 }); }
    catch (err) { console.warn(`Testdaten nicht gelöscht (${dataDir}): ${err.message}`); }
  };
  try {
    // the EXE prints the address it took (another one if the port was taken meanwhile)
    const url = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('App hat sich nicht gemeldet:\n' + output)), 20000);
      const read = chunk => {
        output += chunk;
        const m = /http:\/\/localhost:\d+/.exec(output);
        if (m) { clearTimeout(timer); resolve(m[0]); }
      };
      child.stdout.setEncoding('utf8').on('data', read);
      child.stderr.setEncoding('utf8').on('data', read);
      exited.then(code => { clearTimeout(timer); reject(new Error(`App beendet (${code}):\n${output}`)); });
    });
    for (let i = 0; ; i++) {
      try { if ((await fetch(url + '/api/setup')).ok) break; } catch {}
      if (i > 100) throw new Error('App antwortet nicht: ' + url);
      await new Promise(r => setTimeout(r, 100));
    }
    return { url, dataDir, apiUrl: fake?.url, stop, log: () => output };
  } catch (err) {
    await stop();
    throw err;
  }
}
