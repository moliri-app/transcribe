// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { buildExe, buildFake } from './app.mjs';

// builds the EXE once from this checkout (or checks TRANSCRIBE_EXE); the web UI is embedded in it.
// Then the stand-in for ElevenLabs, for the tests that use it.
export default function globalSetup() {
  buildExe();
  buildFake();
}
