// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { TESTDATA } from '../app.mjs';
import { expect, KORRIGIERT, test, transcript } from './fixtures';

type Page = import('@playwright/test').Page;
const shelf = (page: Page) => page.getByRole('list', { name: 'Transkribierte Aufnahmen' }).locator(':scope > li');
const card = (page: Page, title: string) => shelf(page).filter({ has: page.getByRole('heading', { name: title, exact: true }) });
const areas = (page: Page) => page.getByRole('navigation', { name: 'Bereiche der Bibliothek' });

// A folder as a team shares it: „Berger.mp3“ in a subfolder, with the transcript a colleague made
// next to it, and „Rundgang.mp3“ that nobody has transcribed yet.
function teamFolder() {
  const folder = join(mkdtempSync(join(tmpdir(), 'tx-ordner-')), 'Team-Ordner');
  mkdirSync(join(folder, 'Interviews'), { recursive: true });
  const berger = join(folder, 'Interviews', 'Berger.mp3');
  copyFileSync(join(TESTDATA, 'audio', `${KORRIGIERT}.mp3`), berger);
  copyFileSync(join(TESTDATA, 'audio', 'Neue Aufnahme.mp3'), join(folder, 'Rundgang.mp3'));
  const results = join(TESTDATA, 'results', KORRIGIERT);
  const part = (name: string) => JSON.parse(readFileSync(join(results, name), 'utf8'));
  writeFileSync(`${berger}.transcribe.json`, JSON.stringify({
    format: 'transcribe-by-moliri', version: 1, audio: 'Berger.mp3', saved: '2026-11-01T10:00:00Z',
    final: part('_final.json'), machine: part('_machine.json'), annotations: part('_annotations.json'),
  }));
  return { folder, berger, sidecar: `${berger}.transcribe.json` };
}

async function connect(page: Page, folder: string) {
  await page.goto('/');
  await areas(page).getByRole('button', { name: 'Ordner verbinden' }).click();
  await page.getByLabel('Pfad des Ordners').fill(folder);
  await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
  await expect(page.getByRole('status')).toHaveText('„Team-Ordner“ ist verbunden: 2 Aufnahmen.');
}

test.describe('Verbundene Ordner', () => {
  test('ein Ordner bringt seine Aufnahmen mit, ohne Kopie; ein Transkript daneben ist sofort da', async ({ page, app }) => {
    const { folder } = teamFolder();
    await connect(page, folder);

    await expect(areas(page).getByRole('button', { name: /Team-Ordner/ })).toHaveAttribute('aria-current', 'page');
    const berger = card(page, 'Berger');
    await expect(berger).toContainText('Interviews');
    await expect(berger).toContainText('1 Korrektur');
    await expect(berger.getByRole('list', { name: 'Sprecher' })).toContainText('Interviewerin');
    await expect(page.getByRole('checkbox', { name: /^Rundgang\.mp3/ })).toBeVisible();
    // the transcripts of the data folder are not in the folder's area
    await expect(card(page, KORRIGIERT)).toHaveCount(0);
    expect(readdirSync(join(app.dataDir, 'audio'))).not.toContain('Berger.mp3');

    await berger.getByRole('link').click();
    await expect(page).toHaveURL(/\/editor$/);
    await expect(transcript(page).getByRole('button', { name: 'Interviewerin', exact: true }).first()).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Aufnahme wählen' }).getByRole('option', { name: 'Berger', exact: true }))
      .toHaveJSProperty('selected', true);
  });

  test('Korrekturen gehen ins Transkript neben der Aufnahme', async ({ page }) => {
    const { folder, sidecar } = teamFolder();
    await connect(page, folder);
    const index = await (await page.request.get('/api/index')).json();
    const stem = index.find((a: { title: string }) => a.title === 'Berger').stem;
    const ann = JSON.parse(readFileSync(sidecar, 'utf8')).annotations;
    ann.speakers['0'] = { ...ann.speakers['0'], name: 'Frau Kühn' };
    const r = await page.request.post(`/api/annotations?stem=${encodeURIComponent(stem)}`, { data: ann });
    expect(r.ok()).toBeTruthy();
    await expect.poll(() => JSON.parse(readFileSync(sidecar, 'utf8')).annotations.speakers['0'].name, { timeout: 10_000 })
      .toBe('Frau Kühn');
  });

  test('trennen nimmt den Ordner aus der Bibliothek und lässt alles darin liegen', async ({ page }) => {
    const { folder, berger, sidecar } = teamFolder();
    await connect(page, folder);
    await page.getByRole('button', { name: 'Ordner trennen' }).click();
    await expect(page.getByRole('status')).toContainText('ist getrennt');
    await expect(areas(page).getByRole('button', { name: /Team-Ordner/ })).toHaveCount(0);
    await expect(card(page, 'Berger')).toHaveCount(0);
    expect(existsSync(berger) && existsSync(sidecar)).toBeTruthy();
  });

  test('ein Pfad, den es nicht gibt, wird nicht verbunden', async ({ page }) => {
    await page.goto('/');
    await areas(page).getByRole('button', { name: 'Ordner verbinden' }).click();
    await page.getByLabel('Pfad des Ordners').fill(join(tmpdir(), 'gibt-es-nicht-4711'));
    await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('Diesen Ordner gibt es nicht');
  });
});
