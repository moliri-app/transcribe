// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { defineConfig } from '@playwright/test';

// Every test gets its own EXE on a free port with a fresh copy of testdata/ (see tests/fixtures.ts),
// so tests run in parallel and never see each other's corrections.
export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  timeout: 45_000,
  expect: { timeout: 8_000 },
  fullyParallel: true,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'report' }]],
  outputDir: 'test-results',
  use: {
    browserName: 'chromium',
    locale: 'de-DE',
    viewport: { width: 1440, height: 900 },
    acceptDownloads: true,
    trace: 'retain-on-failure',
    launchOptions: { args: ['--autoplay-policy=no-user-gesture-required', '--disable-audio-output'] },
  },
});
