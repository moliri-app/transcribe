// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { annotationsOf, corrections, expect, INTERVIEW, KORRIGIERT, openEditor, saveState, selectWord, test, transcript, word } from './fixtures';

test.describe('Text ersetzen und einfügen', () => {
  test('ein falsch erkanntes Wort überschreiben: als Korrektur vermerkt, gespeichert, nach dem Neuladen noch da', async ({ page, app }) => {
    await openEditor(page);
    await expect(saveState(page)).toHaveText('noch keine Korrekturen');
    await selectWord(page, 'Habaga');
    await page.keyboard.type('Herr Berger');
    await page.keyboard.press('Enter');

    await expect(word(page, 'Habaga')).toHaveCount(0);
    await expect(word(page, 'Herr')).toBeVisible();
    await expect(word(page, 'Berger')).toBeVisible();
    await expect(corrections(page).getByRole('listitem')).toHaveCount(1);
    await expect(corrections(page)).toContainText('ersetzt: Herr Berger');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    await word(page, 'Berger').click();
    await expect(page.getByRole('toolbar', { name: 'Stelle im Text' })).toContainText('von Hand korrigiert');

    const edits = annotationsOf(app, INTERVIEW).edits;
    expect(edits).toEqual([{ start: 2.166, end: 2.74, text: 'Herr Berger' }]);

    await page.reload();
    await openEditor(page);
    await expect(word(page, 'Habaga')).toHaveCount(0);
    await expect(word(page, 'Berger')).toBeVisible();
    await expect(saveState(page)).toHaveText('alle Korrekturen gespeichert');
  });

  test('zwischen zwei Wörtern tippen fügt ein, ohne Nachbarwörter zu ändern', async ({ page, app }) => {
    await openEditor(page);
    await selectWord(page, 'Sie');
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.type('ja, ');
    await page.keyboard.press('Enter');

    await expect(word(page, 'ja,')).toBeVisible();
    await expect(word(page, 'Habaga')).toBeVisible();
    await expect(word(page, 'Sie')).toBeVisible();
    await expect(corrections(page)).toContainText('eingefügt: ja,');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    const [edit] = annotationsOf(app, INTERVIEW).edits;
    expect(edit).toMatchObject({ text: 'ja,', mode: 'insert' });
    expect(edit.start).toBeGreaterThanOrEqual(2.166);
    expect(edit.end).toBeLessThanOrEqual(3.462);
  });

  test('eine Tipppause übernimmt und speichert ohne Enter; weitertippen geht an derselben Stelle', async ({ page, app }) => {
    await openEditor(page);
    await selectWord(page, 'angefangen?');
    await page.keyboard.press('ArrowLeft');
    // mitten im Wort wartet die Übernahme, damit es nicht mit „angefangen?“ verschmilzt
    await page.keyboard.type('eigentl');
    await page.waitForTimeout(2500);
    await expect(saveState(page)).toContainText('Eingabe offen');

    await page.keyboard.type('ich ');
    await expect(saveState(page)).toHaveText(/^gespeichert/, { timeout: 10_000 });
    expect(annotationsOf(app, INTERVIEW).edits).toMatchObject([{ text: 'eigentlich', mode: 'insert' }]);
    await expect(word(page, 'angefangen?')).toBeVisible();

    // die Schreibmarke steht nach dem Übernehmen noch hinter „eigentlich “; was dort weitergetippt
    // wird, wächst in dieselbe Einfügung
    await page.keyboard.type('schon ');
    await expect(saveState(page)).toHaveText(/^gespeichert/, { timeout: 10_000 });
    await expect(transcript(page)).toContainText('Wie hat das eigentlich schon angefangen?');
    await expect(corrections(page).getByRole('listitem')).toHaveCount(1);
    expect(annotationsOf(app, INTERVIEW).edits).toMatchObject([{ text: 'eigentlich schon', mode: 'insert' }]);
  });

  test('Esc verwirft eine offene Eingabe', async ({ page }) => {
    await openEditor(page);
    await selectWord(page, 'Habaga');
    await page.keyboard.type('Unsinn');
    await expect(saveState(page)).toContainText('Eingabe offen');
    await page.keyboard.press('Escape');
    await expect(word(page, 'Habaga')).toBeVisible();
    await expect(word(page, 'Unsinn')).toHaveCount(0);
    await expect(saveState(page)).toHaveText('Eingabe verworfen');
    await expect(corrections(page).getByRole('listitem')).toHaveCount(0);
  });

  test('eine gespeicherte Korrektur lässt sich in der Liste entfernen', async ({ page, app }) => {
    await openEditor(page, KORRIGIERT);
    await expect(word(page, 'Habaga')).toHaveCount(0);
    const item = corrections(page).getByRole('listitem').filter({ hasText: 'Herr Berger' });
    await item.getByRole('button', { name: 'Korrektur entfernen' }).click();
    await expect(word(page, 'Habaga')).toBeVisible();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, KORRIGIERT).edits).toEqual([]);
  });
});

test.describe('Rückgängig', () => {
  test('der Knopf und Strg+Z nehmen die letzte Korrektur zurück, auch gespeichert', async ({ page, app }) => {
    await openEditor(page);
    const undo = page.getByRole('button', { name: 'Rückgängig' });
    await expect(undo).toBeDisabled();

    await selectWord(page, 'Habaga');
    await page.keyboard.type('Herr Berger');
    await page.keyboard.press('Enter');
    await expect(word(page, 'Berger')).toBeVisible();
    await undo.click();
    await expect(word(page, 'Habaga')).toBeVisible();
    await expect(word(page, 'Berger')).toHaveCount(0);
    await expect(corrections(page).getByRole('listitem')).toHaveCount(0);
    await expect(undo).toBeDisabled();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, INTERVIEW).edits).toEqual([]);

    await selectWord(page, 'Lehrling');
    await page.keyboard.type('Lehrjunge');
    await page.keyboard.press('Enter');
    await expect(word(page, 'Lehrjunge')).toBeVisible();
    await page.keyboard.press('Control+z');
    await expect(word(page, 'Lehrling')).toBeVisible();
    await expect(word(page, 'Lehrjunge')).toHaveCount(0);
  });

  test('Rückgängig nimmt auch Umbenennen und Clips zurück, Schritt für Schritt', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    const undo = page.getByRole('button', { name: 'Rückgängig' });
    const clips = page.getByRole('list', { name: 'Clips' }).getByRole('listitem');
    await expect(clips).toHaveCount(2);
    await clips.filter({ hasText: 'Filze waschen' }).getByRole('button', { name: 'Filze waschen bearbeiten' }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Löschen' }).click();
    await expect(clips).toHaveCount(1);

    await page.getByRole('list', { name: 'Sprecher' }).getByRole('button', { name: 'Herr Berger', exact: true }).click();
    await page.getByRole('dialog').getByLabel('Name').fill('Karl Berger');
    await page.getByRole('dialog').getByRole('button', { name: 'Übernehmen' }).click();
    await expect(page.getByRole('list', { name: 'Sprecher' })).toContainText('Karl Berger');

    await undo.click();
    await expect(page.getByRole('list', { name: 'Sprecher' })).not.toContainText('Karl Berger');
    await expect(clips).toHaveCount(1);
    await undo.click();
    await expect(clips).toHaveCount(2);
  });
});
