// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { statSync } from "node:fs";
import { annotationsOf, clipList, dialog, downloadText, dragSelect, expect, heardWords, INTERVIEW, KORRIGIERT, openEditor, playButton, saveState, test, waitHeard, word } from './fixtures';

test.describe('Clips anlegen und abspielen', () => {
  test('eine Stelle im Text markieren und als Clip mit Titel und Schlagworten anlegen', async ({ page, app }) => {
    await openEditor(page);
    await expect(clipList(page)).toContainText('Noch keine');
    await dragSelect(page, word(page, 'musste'), word(page, 'waschen'));
    await expect(dialog(page)).toContainText('musste die Filze waschen');
    await dialog(page).getByRole('button', { name: 'Als Clip' }).click();

    await expect(dialog(page)).toContainText('Neuer Clip');
    await dialog(page).getByLabel('Titel').fill('Filze waschen');
    await dialog(page).getByLabel('Schlagworte').fill('Arbeit, Fabrik');
    await dialog(page).getByRole('button', { name: 'Clip anlegen' }).click();
    await expect(dialog(page)).toBeHidden();

    const item = clipList(page).getByRole('listitem');
    await expect(item).toHaveCount(1);
    await expect(item).toContainText('Filze waschen');
    await expect(item).toContainText('Arbeit');
    await expect(item).toContainText('Fabrik');
    await expect(item).toContainText('0:23 – 0:24');
    await expect(saveState(page)).toHaveText(/^gespeichert/);

    const [clip] = annotationsOf(app, INTERVIEW).clips;
    expect(clip).toMatchObject({ title: 'Filze waschen', tags: ['Arbeit', 'Fabrik'] });
    expect(clip.start).toBeGreaterThan(23.1);
    expect(clip.start).toBeLessThan(23.3);
    expect(clip.end).toBeGreaterThan(24.6);
    expect(clip.end).toBeLessThan(24.9);
  });

  test('ein Clip ohne Titel bekommt die ersten Wörter als Titel; falsche Zeiten werden abgelehnt', async ({ page }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'Was'), word(page, 'Aufgabe?'));
    await dialog(page).getByRole('button', { name: 'Als Clip' }).click();
    await dialog(page).getByLabel('Ende').fill('0:10');
    await dialog(page).getByRole('button', { name: 'Clip anlegen' }).click();
    await expect(dialog(page)).toContainText('das Ende muss nach dem Anfang liegen');
    await dialog(page).getByLabel('Ende').fill('0:21,6');
    await dialog(page).getByRole('button', { name: 'Clip anlegen' }).click();
    await expect(clipList(page).getByRole('listitem')).toHaveText(/Was war ihre erste Aufgabe/);
  });

  test('▶ spielt nur diesen Clip und hält an seinem Ende an', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    const clip = clipList(page).getByRole('listitem').filter({ hasText: 'Filze waschen' });
    await clip.getByRole('button', { name: 'nur diesen Clip abspielen' }).click();
    await expect(clip.getByRole('button', { name: 'anhalten' })).toBeVisible();
    await expect(playButton(page)).toHaveAccessibleName(/^Anhalten/);
    // the clip runs 23.0–26.1 s: it stops by itself after "beschriften."
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/, { timeout: 10_000 });
    const seen = await heardWords(page);
    expect(seen).toEqual(expect.arrayContaining(['Filze', 'waschen', 'beschriften.']));
    expect(seen).not.toContain('Arbeit.');
    expect(seen).not.toContain('Aufgabe?');
  });

  test('ein Klick auf den Titel springt zum Clip, ohne abzuspielen', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    await clipList(page).getByRole('button', { name: 'Filze waschen', exact: true }).click();
    await expect(word(page, 'Filze')).toBeInViewport();
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/);
    await playButton(page).click();
    await waitHeard(page, 'waschen');
    expect((await heardWords(page))[0]).toMatch(/^(Ich|musste|die|Filze)$/);
  });

  test('ausgewählte Clips nacheinander abspielen: dazwischen wird übersprungen', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    await expect(clipList(page).getByRole('listitem')).toHaveCount(2);
    await page.getByRole('checkbox', { name: 'alle auswählen' }).check();
    await expect(page.getByText('2 ausgewählt')).toBeVisible();
    await page.getByRole('combobox', { name: 'Geschwindigkeit' }).selectOption({ label: '2×' });
    await page.getByRole('button', { name: 'Nacheinander abspielen' }).click();
    await waitHeard(page, 'beschriften.', 15_000);
    await expect(playButton(page)).toHaveAccessibleName(/^Abspielen/, { timeout: 10_000 });
    const seen = await heardWords(page);
    expect(seen).toEqual(expect.arrayContaining(['Lehrling', 'Papiermaschine.', 'Filze']));
    expect(seen.indexOf('Lehrling')).toBeLessThan(seen.indexOf('Filze'));
    for (const between of ['Was', 'Aufgabe?', 'Arbeit.']) expect(seen).not.toContain(between);
  });

  test('nach Schlagwort filtern', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    const items = clipList(page).getByRole('listitem');
    await expect(items).toHaveCount(2);
    await page.getByRole('button', { name: 'Arbeit', exact: true }).click();
    await expect(items).toHaveCount(1);
    await expect(items).toContainText('Filze waschen');
    await page.getByRole('button', { name: 'Arbeit', exact: true }).click();
    await expect(items).toHaveCount(2);
  });

  test('einen Clip ändern und löschen', async ({ page, app }) => {
    await openEditor(page, KORRIGIERT);
    await clipList(page).getByRole('button', { name: 'Anfang als Lehrling bearbeiten' }).click();
    await dialog(page).getByLabel('Titel').fill('Lehrzeit');
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(clipList(page).getByRole('button', { name: 'Lehrzeit', exact: true })).toBeVisible();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, KORRIGIERT).clips.map((c: any) => c.title)).toEqual(['Lehrzeit', 'Filze waschen']);

    await clipList(page).getByRole('button', { name: 'Lehrzeit bearbeiten' }).click();
    await dialog(page).getByRole('button', { name: 'Löschen' }).click();
    await expect(clipList(page).getByRole('listitem')).toHaveCount(1);
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, KORRIGIERT).clips.map((c: any) => c.title)).toEqual(['Filze waschen']);
  });
});

test.describe('Clips exportieren', () => {
  test('einen Clip als Text und als Audio herunterladen', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    await clipList(page).getByRole('button', { name: 'Filze waschen exportieren' }).click();
    let wait = page.waitForEvent('download');
    await dialog(page).getByRole('button', { name: /Text mit Titel und Schlagworten/ }).click();
    let d = await wait;
    expect(d.suggestedFilename()).toMatch(/Filze waschen.*\.txt$/);
    const txt = await downloadText(d);
    expect(txt).toContain('Filze waschen');
    expect(txt).toContain('Arbeit');
    expect(txt).toContain('Ich musste die Filze waschen');
    await expect(page.getByRole('status')).toContainText('exportiert');

    await clipList(page).getByRole('button', { name: 'Filze waschen exportieren' }).click();
    wait = page.waitForEvent('download');
    await dialog(page).getByRole('button', { name: 'Audio (MP3)' }).click();
    d = await wait;
    expect(d.suggestedFilename()).toMatch(/\.mp3$/);
    expect(statSync((await d.path())!).size).toBeGreaterThan(5000);
  });

  test('mehrere Clips als Zusammenschnitt exportieren', async ({ page }) => {
    await openEditor(page, KORRIGIERT);
    await page.getByRole('checkbox', { name: 'Anfang als Lehrling auswählen' }).check();
    await page.getByRole('checkbox', { name: 'Filze waschen auswählen' }).check();
    await page.getByRole('button', { name: 'Zusammenschnitt exportieren' }).click();
    await expect(dialog(page)).toContainText('Zusammenschnitt aus 2 Clips');
    const wait = page.waitForEvent('download');
    await dialog(page).getByRole('button', { name: /Text mit Titel und Schlagworten/ }).click();
    const txt = await downloadText(await wait);
    expect(txt.indexOf('Anfang als Lehrling')).toBeGreaterThanOrEqual(0);
    expect(txt.indexOf('Anfang als Lehrling')).toBeLessThan(txt.indexOf('Filze waschen'));
  });
});
