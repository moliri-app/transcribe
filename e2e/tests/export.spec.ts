// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import type { Page } from '@playwright/test';
import { downloadText, expect, KORRIGIERT, openEditor, saveState, selectWord, test } from './fixtures';

async function exportAs(page: Page, format: string) {
  await page.getByRole('button', { name: 'Transkript mit allen Korrekturen herunterladen' }).click();
  const wait = page.waitForEvent('download');
  await page.getByRole('button', { name: format }).click();
  return wait;
}

test.describe('Export', () => {
  test('als Text: mit Sprechernamen, Zeitmarken und allen Korrekturen', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    const d = await exportAs(page, 'Text (.txt)');
    expect(d.suggestedFilename()).toBe('Papierfabrik mit Korrekturen.txt');
    const txt = await downloadText(d);
    expect(txt).toContain('Interviewerin: [00:00:00] Guten Tag. Herr Berger Sie haben 40 Jahre');
    expect(txt).toContain('Herr Berger: [00:00:10] Angefangen hat es im Sommer 1960.');
    expect(txt).not.toContain('Habaga');
  });

  test('als Untertitel (.vtt) und als HTML', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    const vtt = await downloadText(await exportAs(page, 'Untertitel (.vtt)'));
    expect(vtt).toMatch(/^WEBVTT/);
    expect(vtt).toContain('00:00:10.334 --> ');
    expect(vtt).toContain('<v Herr Berger>Angefangen hat es');

    const d = await exportAs(page, /^HTML/);
    expect(d.suggestedFilename()).toMatch(/\.html?$/);
    const html = await downloadText(d);
    expect(html).toContain('<title>Papierfabrik mit Korrekturen</title>');
    expect(html).toContain('Interviewerin:');
  });

  test('eine gerade getippte Korrektur ist im Export', async ({ page }) => {
    await openEditor(page);
    await selectWord(page, 'Habaga');
    await page.keyboard.type('Herr Berger');
    await page.keyboard.press('Enter');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    const txt = await downloadText(await exportAs(page, 'Text (.txt)'));
    expect(txt).toContain('Guten Tag. Herr Berger Sie haben');
  });
});
