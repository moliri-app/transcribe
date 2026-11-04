// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { annotationsOf, chooseLanguage, corrections, dialog, dragSelect, expect, INTERVIEW, openEditor, saveState, speakerList, test, transcript, word } from './fixtures';

test.describe('Sprecher benennen und zusammenlegen', () => {
  test('einen Sprecher benennen: überall im Text und in der Liste, gespeichert', async ({ page, app }) => {
    await openEditor(page);
    await expect(speakerList(page).getByRole('listitem')).toHaveCount(2);
    await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
    await expect(dialog(page)).toBeVisible();
    await dialog(page).getByLabel('Name').fill('Herr Berger');
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(dialog(page)).toBeHidden();

    await expect(speakerList(page).getByRole('button', { name: 'Herr Berger', exact: true })).toBeVisible();
    await expect(transcript(page).getByRole('button', { name: 'Herr Berger', exact: true })).toHaveCount(2);
    await expect(transcript(page).getByRole('button', { name: 'Sprecher 2', exact: true })).toHaveCount(0);
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, INTERVIEW).speakers).toEqual({ 1: { name: 'Herr Berger' } });

    await page.reload();
    await expect(transcript(page).getByRole('button', { name: 'Herr Berger', exact: true }).first()).toBeVisible();
  });

  test('einen Namen im Text anklicken öffnet denselben Sprecher; Abbrechen ändert nichts', async ({ page, app }) => {
    await openEditor(page);
    await transcript(page).getByRole('button', { name: 'Sprecher 1', exact: true }).first().click();
    await dialog(page).getByLabel('Name').fill('Interviewerin');
    await dialog(page).getByRole('button', { name: 'Abbrechen' }).click();
    await expect(dialog(page)).toBeHidden();
    await expect(speakerList(page).getByRole('button', { name: 'Interviewerin' })).toHaveCount(0);

    await transcript(page).getByRole('button', { name: 'Sprecher 1', exact: true }).first().click();
    await dialog(page).getByLabel('Name').fill('Interviewerin');
    await page.keyboard.press('Enter');
    await expect(speakerList(page).getByRole('button', { name: 'Interviewerin', exact: true })).toBeVisible();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, INTERVIEW).speakers).toEqual({ 0: { name: 'Interviewerin' } });
  });

  test('zwei Sprecher zusammenlegen: vorgemerkt, nach „Neu berechnen“ bleibt einer', async ({ page, app }) => {
    await openEditor(page);
    const recompute = page.getByRole('button', { name: 'Neu berechnen' });
    await expect(recompute).toBeDisabled();

    await speakerList(page).getByRole('button', { name: 'Sprecher 1', exact: true }).click();
    await dialog(page).getByRole('combobox', { name: 'Zusammenlegen' }).selectOption({ label: 'Sprecher 2' });
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(speakerList(page).getByRole('listitem').filter({ hasText: 'Sprecher 1' })).toContainText('wird zusammengelegt mit Sprecher 2');
    await expect(recompute).toBeEnabled();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, INTERVIEW).speakers).toEqual({ 0: { merge_into: '1' } });

    await recompute.click();
    await expect(speakerList(page).getByRole('listitem')).toHaveCount(1);
    await expect(speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true })).toBeVisible();
    await expect(transcript(page).getByRole('button', { name: 'Sprecher 1', exact: true })).toHaveCount(0);
    await expect(recompute).toBeDisabled();
    const final = app.json(`results/${INTERVIEW}/_final.json`);
    const speakers = new Set(final.segments.flatMap((s: any) => s.words.map((w: any) => w.speaker)));
    expect([...speakers]).toEqual([1]);
  });
});

test.describe('Sprache pro Sprecher oder Stelle', () => {
  test('Sprache für einen Sprecher: am Namen und in der Liste, vorgemerkt für „Neu berechnen“', async ({ page, app }) => {
    await openEditor(page);
    const recompute = page.getByRole('button', { name: 'Neu berechnen' });
    await expect(recompute).toBeDisabled();
    await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
    await chooseLanguage(page, 'Englisch');
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(dialog(page)).toBeHidden();

    await expect(speakerList(page).getByRole('listitem').filter({ hasText: 'Sprecher 2' })).toContainText('Englisch');
    await expect(transcript(page).getByRole('button', { name: /^Sprecher 2\s*Englisch$/ }).first()).toBeVisible();
    await expect(recompute).toBeEnabled();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    expect(annotationsOf(app, INTERVIEW).speakers).toEqual({ 1: { language: 'en' } });

    // die Wortleiste nennt die Sprache als von Hand festgelegt
    await word(page, 'Sommer').click();
    const bar = page.getByRole('toolbar', { name: 'Stelle im Text' });
    await expect(bar).toContainText('Englisch');
    await expect(bar).toContainText('von Hand');
  });

  test('Sprache für eine Stelle: als Korrektur vermerkt, nur dort', async ({ page, app }) => {
    await openEditor(page);
    await dragSelect(page, word(page, 'angefangen?'), word(page, 'Sommer'));
    await chooseLanguage(page, 'Englisch');
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(dialog(page)).toBeHidden();

    await expect(corrections(page).getByRole('listitem')).toHaveCount(1);
    await expect(corrections(page)).toContainText('Englisch');
    await expect(page.getByRole('button', { name: 'Neu berechnen' })).toBeEnabled();
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    const ann = annotationsOf(app, INTERVIEW);
    expect(ann.speakers).toEqual({});
    expect(ann.spans).toHaveLength(1);
    expect(ann.spans[0]).toMatchObject({ language: 'en' });
    expect(ann.spans[0]).not.toHaveProperty('speaker');

    const bar = page.getByRole('toolbar', { name: 'Stelle im Text' });
    await word(page, 'Sommer').click();
    await expect(bar).toContainText('Englisch');
    await word(page, 'Gespräch.').click();
    await expect(bar).toContainText('Deutsch');
  });
});

test.describe('Neu berechnen', () => {
  test('zurückgenommen, bevor gerechnet ist: nichts mehr vorgemerkt', async ({ page }) => {
    await openEditor(page);
    const recompute = page.getByRole('button', { name: 'Neu berechnen' });
    // ein Name wirkt sofort und braucht kein Neuberechnen
    await speakerList(page).getByRole('button', { name: 'Sprecher 1', exact: true }).click();
    await dialog(page).getByLabel('Name').fill('Interviewerin');
    await page.keyboard.press('Enter');
    await expect(saveState(page)).toHaveText(/^gespeichert/);
    await expect(recompute).toBeDisabled();

    await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
    await chooseLanguage(page, 'Englisch');
    await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
    await expect(recompute).toBeEnabled();
    // der Name kommt dazu; Rückgängig nimmt ihn und dann die Sprache zurück
    await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
    await dialog(page).getByLabel('Name').fill('Herr Berger');
    await page.keyboard.press('Enter');
    await page.getByRole('button', { name: 'Rückgängig' }).click();
    await expect(recompute).toBeEnabled();
    await page.getByRole('button', { name: 'Rückgängig' }).click();
    await expect(recompute).toBeDisabled();
  });
});

test('Sprache suchen: „Platt“ findet Plattdeutsch, das nur vermerkt wird', async ({ page, app }) => {
  await openEditor(page);
  await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
  const field = dialog(page).getByRole('combobox', { name: 'Sprache' });
  await field.fill('Platt');
  const option = dialog(page).getByRole('option', { name: /^Plattdeutsch/ });
  await expect(option).toContainText('nds');
  await expect(option).toContainText('nicht neu erkannt');
  await field.press('Enter');
  await expect(field).toHaveValue('Plattdeutsch (nds)');
  await expect(dialog(page)).toBeVisible();
  await dialog(page).getByRole('button', { name: 'Übernehmen' }).click();
  await expect(saveState(page)).toHaveText(/^gespeichert/);
  expect(annotationsOf(app, INTERVIEW).speakers).toEqual({ 1: { language: 'nds' } });
  await expect(speakerList(page).getByRole('listitem').filter({ hasText: 'Sprecher 2' })).toContainText('Plattdeutsch');
  // die Maschine kann Plattdeutsch nicht neu erkennen: nichts vorgemerkt
  await expect(page.getByRole('button', { name: 'Neu berechnen' })).toBeDisabled();

  // der Code allein findet dieselbe Sprache; beim nächsten Öffnen steht sie mit Namen da
  await speakerList(page).getByRole('button', { name: 'Sprecher 2', exact: true }).click();
  await expect(field).toHaveValue('Plattdeutsch (nds)');
  await expect(dialog(page).getByLabel('Name')).toBeFocused();
  await field.fill('pt');
  await expect(dialog(page).getByRole('option').first()).toContainText('Portugiesisch');
  await field.press('Escape');
  await expect(dialog(page)).toBeVisible();
  await expect(field).toHaveValue('Plattdeutsch (nds)');
});
