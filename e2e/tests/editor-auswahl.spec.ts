// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { annotationsOf, clipList, corrections, dialog, dragSelect, expect, heardWords, INTERVIEW, openEditor, playButton, saveState, test, waitHeard, word } from './fixtures';

test.describe('Auswahl über Sprecher und Absätze', () => {
  test('eine Auswahl von einem Sprecher in den nächsten zeigt den ganzen Bereich', async ({ page }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'angefangen?'), word(page, 'Sommer'));
    await expect(dialog(page)).toBeVisible();
    await expect(dialog(page)).toContainText('0:08 – 0:12');
    await expect(dialog(page)).toContainText('angefangen? Angefangen hat es im Sommer');
    await dialog(page).getByRole('button', { name: 'Schließen' }).click();
    await expect(dialog(page)).toBeHidden();
  });

  test('der markierte Bereich lässt sich abspielen und endet an seinem Ende', async ({ page }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'Aufgabe?'), word(page, 'musste'));
    await expect(dialog(page)).toContainText('Aufgabe? Ich musste');
    await dialog(page).getByRole('button', { name: 'Abspielen', exact: true }).click();
    await dialog(page).getByRole('button', { name: 'Schließen' }).click();
    await waitHeard(page, 'musste');
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/, { timeout: 8000 });
    expect(await heardWords(page)).not.toContain('Filze');
  });

  test('über drei Absätze: Sprecher für den Bereich festlegen', async ({ page, app }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'Papiermaschine.'), word(page, 'Filze'));
    await expect(dialog(page)).toContainText('Papiermaschine. Was war ihre erste Aufgabe? Ich musste die Filze');
    await dialog(page).getByRole('combobox', { name: 'Sprecher' }).selectOption({ label: 'Sprecher 2' });
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(corrections(page).getByRole('listitem')).toHaveCount(1);
    await expect(corrections(page)).toContainText('→ Sprecher 2');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    const [span] = annotationsOf(app, INTERVIEW).spans;
    expect(span).toMatchObject({ speaker: '1' });
    expect(span.start).toBeCloseTo(17.23, 1);
    expect(span.end).toBeCloseTo(24.32, 1);

    // "Was war ihre erste Aufgabe?" now belongs to Sprecher 2, set by hand
    await word(page, 'Aufgabe?').click();
    await expect(page.getByRole('toolbar', { name: 'Stelle im Text' })).toContainText('Sprecher 2 von Hand');
  });

  test('über Absätze hinweg als Clip anlegen', async ({ page, app }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'angefangen?'), word(page, 'Sommer'));
    await dialog(page).getByRole('button', { name: 'Als Clip' }).click();
    await dialog(page).getByRole('button', { name: 'Clip anlegen' }).click();
    await expect(clipList(page).getByRole('listitem')).toHaveCount(1);
    await expect(clipList(page)).toContainText('angefangen? Angefangen hat es im Sommer');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    const [clip] = annotationsOf(app, INTERVIEW).clips;
    expect(clip.start).toBeCloseTo(8.1, 1);
    expect(clip.end).toBeGreaterThan(12.0);
    expect(clip.end).toBeLessThan(12.3);
  });
});
