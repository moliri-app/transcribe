// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, test } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startApp } from '../app.mjs';

// The data dir lies in the user's profile (LOCALAPPDATA), not next to the EXE. The settings move it
// to another folder: the app copies the data there, starts anew on the same address and removes the
// old folder. TRANSCRIBE_PICK_FOLDER stands in for the folder chosen in the system's dialog.
test.setTimeout(120_000);

test('der Datenordner zieht in einen anderen Ordner um', async ({ page }) => {
  test.skip(!!process.env.TRANSCRIBE_EXE, 'uses the EXE of this checkout');
  const target = mkdtempSync(join(tmpdir(), 'tx-ziel-')); // empty: the data go right into it
  const app = await startApp({ home: true, env: { TRANSCRIBE_PICK_FOLDER: target } });
  try {
    await page.goto(app.url + '/settings#speicher');
    await expect(page.getByRole('heading', { name: 'Ort des Datenordners' })).toBeVisible();
    await expect(page.getByText(app.dataDir, { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Standard', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Zurück an den Standardort' })).toHaveCount(0);

    await page.getByRole('button', { name: 'Anderen Ort wählen …' }).click();
    await expect(page.getByRole('heading', { name: 'Datenordner umziehen?' })).toBeVisible();
    await page.getByRole('button', { name: 'Umziehen' }).click();

    await expect(page.getByRole('status').filter({ hasText: 'Die Daten liegen jetzt in' })).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText('Zurück an den Standardort')).toBeVisible();
    // everything came along, and the old folder went (location.json, which remembers the new one, aside)
    expect(existsSync(join(target, 'settings.json'))).toBe(true);
    expect(JSON.parse(readFileSync(join(target, 'results', '_queue.json'), 'utf8')).jobs).toHaveLength(2);
    await expect.poll(() => existsSync(join(app.dataDir, 'settings.json')), { timeout: 20_000 }).toBe(false);
    expect(JSON.parse(readFileSync(join(app.dataDir, 'location.json'), 'utf8')).dataDir).toBe(target);
    // the recordings are still there
    await page.goto(app.url + '/?neu');
    await expect(page.getByText('Papierfabrik Interview').first()).toBeVisible();
  } finally {
    // the app started anew is no child of this test: its pid is in the new data directory
    try {
      const { pid } = JSON.parse(readFileSync(join(target, 'instance.json'), 'utf8'));
      if (process.platform === 'win32') spawnSync('taskkill', ['/pid', String(pid), '/T', '/F'], { stdio: 'ignore' });
      else process.kill(pid);
    } catch {}
    await app.stop();
    rmSync(target, { recursive: true, force: true, maxRetries: 20, retryDelay: 150 });
  }
});
