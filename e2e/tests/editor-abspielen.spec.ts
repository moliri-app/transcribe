// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, heard, heardWords, openEditor, playButton, speakerList, spoken, test, transcript, waitHeard, word } from './fixtures';

test.describe('Abspielen und Mitlesen', () => {
  test('Abspielen läuft durch den Text, das gesprochene Wort ist markiert; Anhalten hält an', async ({ page }) => {
    await openEditor(page);
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/);
    await playButton(page).click();
    await expect(playButton(page)).toHaveAccessibleName(/^Anhalten/);
    await waitHeard(page, 'Papierfabrik');
    // word after word, in the order of the text (a busy machine may drop a frame and skip one)
    const text = ['Guten', 'Tag.', 'Habaga', 'Sie', 'haben', '40', 'Jahre', 'in', 'der', 'Papierfabrik'];
    const seen = await heardWords(page);
    expect(seen[0]).toBe('Guten');
    expect(seen.length).toBeGreaterThanOrEqual(7);
    const at = seen.map(w => text.indexOf(w));
    expect(at.every((k, i) => k >= 0 && (i === 0 || k > at[i - 1]))).toBe(true);

    await playButton(page).click();
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/);
    await expect(spoken(page)).toHaveCount(0);
  });

  test('ein Klick in den Text zeigt die Stelle; „Ab hier“ spielt ab diesem Wort', async ({ page }) => {
    await openEditor(page);
    await word(page, 'Lehrling').click();
    const bar = page.getByRole('toolbar', { name: 'Stelle im Text' });
    await expect(bar).toBeVisible();
    await expect(bar).toContainText('Sprecher 2');
    await expect(bar).toContainText('Deutsch');

    await bar.getByRole('button', { name: 'Ab hier' }).click();
    await waitHeard(page, 'Papiermaschine.');
    const seen = await heardWords(page);
    expect(seen).toContain('Lehrling');
    expect(seen).not.toContain('Sommer');  // started at the word, not before it
    await bar.getByRole('button', { name: 'Anhalten' }).click();
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/);
  });

  test('Strg+Klick auf ein Wort spielt sofort ab dort, schneller mit der Geschwindigkeit', async ({ page }) => {
    await openEditor(page);
    await page.getByRole('combobox', { name: 'Geschwindigkeit' }).selectOption({ label: '2×' });
    await word(page, 'Filze').click({ modifiers: ['Control'] });
    await waitHeard(page, 'Arbeit.');
    const log = await heard(page);
    expect(log.map(h => h.w)).not.toContain('Aufgabe?');
    // from "Filze" to "Arbeit." are 4 s of the recording: about 2 s at double speed
    const t = (w: string) => log.find(h => h.w === w)!.t;
    expect(t('Arbeit.') - t('Filze')).toBeGreaterThan(1300);
    expect(t('Arbeit.') - t('Filze')).toBeLessThan(3500);
  });

  test('ein ausgeblendeter Sprecher ist eingeklappt und wird beim Abspielen übersprungen', async ({ page }) => {
    await openEditor(page);
    await speakerList(page).getByRole('button', { name: 'Sprecher 2 ausblenden' }).click();
    await expect(word(page, 'Lehrling')).toHaveCount(0);
    await expect(transcript(page).getByRole('button', { name: /eingeklappt/ })).toHaveCount(2);
    await expect(page.getByText('Nur Sprecher 1', { exact: true })).toBeVisible();

    await page.getByRole('combobox', { name: 'Geschwindigkeit' }).selectOption({ label: '2×' });
    await word(page, 'angefangen?').click({ modifiers: ['Control'] });
    await waitHeard(page, 'Aufgabe?');
    const seen = await heardWords(page);
    expect(seen).toContain('angefangen?');
    expect(seen).toContain('Was');
    expect(seen).not.toContain('Angefangen');
    expect(seen).not.toContain('Lehrling');

    await page.getByRole('button', { name: 'alle zeigen' }).click();
    await expect(word(page, 'Lehrling')).toBeVisible();
  });
});
