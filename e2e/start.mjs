// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Builds the EXE and starts it on a free port with a fresh copy of the test data, to try the app by hand
// in the same state the tests see. Ctrl+C stops it and removes the copy.
//   node start.mjs                 (or: pnpm start)
//   node start.mjs --missing whisper-base   (leave a model out: the first-run download fails offline)
//   node start.mjs --api                   (and the stand-in for speech recognition services, internal/sttapi/sttapitest)
import { buildExe, buildFake, startApp } from './app.mjs';

const i = process.argv.indexOf('--missing');
const missing = i > 0 ? process.argv[i + 1].split(',') : [];
const api = process.argv.includes('--api');
const exe = buildExe();
if (api) buildFake();
const app = await startApp({ exe, missing, api });
console.log(`\nTranscribe läuft: ${app.url}\nTestdaten (Kopie): ${app.dataDir}\n${api ? `API-Nachbau: ${app.apiUrl}/v1 (OpenAI-kompatibel), ${app.apiUrl} (ElevenLabs)\n` : ''}Strg+C beendet.\n`);
let stopping = false;
const quit = async () => { if (stopping) return; stopping = true; await app.stop(); process.exit(0); };
process.on('SIGINT', quit);
process.on('SIGTERM', quit);
setInterval(() => {}, 1 << 30);
