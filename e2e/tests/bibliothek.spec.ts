// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { TESTDATA } from '../app.mjs';
import { expect, INTERVIEW, KORRIGIERT, test, transcript } from './fixtures';

type Page = import('@playwright/test').Page;
const jobs = (page: Page) => page.getByRole('list', { name: 'Aufträge' }).getByRole('listitem');
const shelf = (page: Page) => page.getByRole('list', { name: 'Transkribierte Aufnahmen' }).locator(':scope > li');
const card = (page: Page, title: string) => shelf(page).filter({ has: page.getByRole('heading', { name: title, exact: true }) });

test.describe('Bibliothek', () => {
  test('transkribierte Aufnahmen stehen als Blätter da; ein Klick öffnet sie im Editor', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 1, name: 'Bibliothek' })).toBeVisible();
    await expect(shelf(page)).toHaveCount(2);
    const korrigiert = card(page, KORRIGIERT);
    await expect(korrigiert).toContainText('0:35');
    await expect(korrigiert).toContainText('2 Sprecher');
    await expect(korrigiert).toContainText('1 Korrektur');
    await expect(korrigiert).toContainText('Guten Tag.');
    await expect(korrigiert.getByRole('list', { name: 'Sprecher' })).toContainText('Interviewerin');

    // transcribed recordings are not offered again as new ones
    await expect(page.getByRole('checkbox', { name: new RegExp(KORRIGIERT) })).toHaveCount(0);

    await korrigiert.getByRole('link').click();
    await expect(page).toHaveURL(/\/editor$/);
    await expect(transcript(page).getByRole('button', { name: 'Interviewerin', exact: true }).first()).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Aufnahme wählen' }).getByRole('option', { name: KORRIGIERT, exact: true }))
      .toHaveJSProperty('selected', true);
  });

  test('in Arbeit steht nur, was läuft oder gescheitert ist', async ({ page }) => {
    await page.goto('/queue');
    await expect(jobs(page)).toHaveCount(1);
    const failed = jobs(page).filter({ hasText: INTERVIEW });
    await expect(failed).toContainText('Fehler');
    await expect(failed).toContainText('Speicher voll');
    await expect(failed.getByRole('button', { name: 'Erneut' })).toBeVisible();
  });

  test('eine neue Aufnahme mit Sprechern transkribieren; der Auftrag erscheint und lässt sich entfernen', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3/ }).check();
    // no preset and no thread count: speakers and languages are what there is to choose
    await expect(page.getByRole('combobox', { name: 'Voreinstellung' })).toHaveCount(0);
    await expect(page.getByRole('spinbutton', { name: 'Threads' })).toHaveCount(0);
    await page.getByRole('combobox', { name: 'Sprecher' }).selectOption('2');
    await page.getByRole('button', { name: 'Transkribieren', exact: true }).click();
    await expect(page.getByRole('status')).toHaveText('Eine Aufnahme wird transkribiert.');

    const job = jobs(page).filter({ hasText: 'Neue Aufnahme' });
    await expect(job).toHaveCount(1);
    await expect(job).toContainText('2 Sprecher');
    // the test data has no real recognition models, so the job ends with an error; then it can be removed
    await expect(job).toContainText('Fehler', { timeout: 15_000 });
    await job.getByRole('button', { name: 'Entfernen' }).click();
    await expect(job).toHaveCount(0);
    await expect(page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3/ })).toBeVisible();
  });

  test('ohne Auswahl wird nichts transkribiert', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Transkribieren', exact: true }).click();
    await expect(page.getByRole('status')).toHaveText('Bitte mindestens eine Aufnahme wählen.');
    await expect(jobs(page)).toHaveCount(1);
  });

  test('eine Datei hinzufügen: sie steht danach angekreuzt bei den neuen Aufnahmen', async ({ page, app }) => {
    await page.goto('/');
    await page.getByLabel('Dateien hinzufügen').setInputFiles({
      name: 'Hochgeladen Test.mp3', mimeType: 'audio/mpeg', buffer: readFileSync(join(TESTDATA, 'audio', 'Neue Aufnahme.mp3')),
    });
    await expect(page.getByRole('status')).toContainText('Hochgeladen.');
    await expect(page.getByRole('checkbox', { name: /^Hochgeladen Test\.mp3/ })).toBeChecked();
    expect(readFileSync(join(app.dataDir, 'audio', 'Hochgeladen Test.mp3')).length).toBeGreaterThan(1000);
  });

  test('eine noch nicht transkribierte Aufnahme löschen, nach einer Rückfrage', async ({ page, app }) => {
    await page.goto('/');
    const row = page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3/ });
    await page.getByRole('button', { name: 'Neue Aufnahme löschen' }).click();
    const ask = page.getByRole('dialog', { name: '„Neue Aufnahme“ löschen?' });
    await expect(ask).toContainText('von diesem Computer gelöscht');
    await expect(ask).toContainText('nicht rückgängig');

    // cancelled: nothing happens
    await ask.getByRole('button', { name: 'Abbrechen' }).click();
    await expect(ask).toBeHidden();
    await expect(row).toBeVisible();
    expect(existsSync(join(app.dataDir, 'audio', 'Neue Aufnahme.mp3'))).toBe(true);

    await page.getByRole('button', { name: 'Neue Aufnahme löschen' }).click();
    await ask.getByRole('button', { name: 'Aufnahme löschen' }).click();
    await expect(ask).toBeHidden();
    await expect(page.getByRole('status')).toHaveText('„Neue Aufnahme“ ist gelöscht.');
    await expect(row).toHaveCount(0);
    expect(existsSync(join(app.dataDir, 'audio', 'Neue Aufnahme.mp3'))).toBe(false);
  });

  test('eine transkribierte Aufnahme löschen: die Rückfrage sagt, dass Transkript und Korrekturen mitgehen', async ({ page, app }) => {
    await page.goto('/');
    await expect(shelf(page)).toHaveCount(2);
    await card(page, KORRIGIERT).getByRole('button', { name: `${KORRIGIERT} löschen` }).click();
    const ask = page.getByRole('dialog', { name: `„${KORRIGIERT}“ löschen?` });
    await expect(ask).toContainText('das Transkript samt allen Korrekturen, Sprechernamen und Clips');
    await expect(ask).toContainText('1 Korrektur von Hand und 2 Clips');

    // Escape closes the question like „Abbrechen“
    await page.keyboard.press('Escape');
    await expect(ask).toBeHidden();
    await expect(shelf(page)).toHaveCount(2);

    await card(page, KORRIGIERT).getByRole('button', { name: `${KORRIGIERT} löschen` }).click();
    await ask.getByRole('button', { name: 'Aufnahme und Transkript löschen' }).click();
    await expect(page.getByRole('status')).toHaveText(`„${KORRIGIERT}“ ist gelöscht.`);
    await expect(shelf(page)).toHaveCount(1);
    await expect(card(page, KORRIGIERT)).toHaveCount(0);
    expect(existsSync(join(app.dataDir, 'audio', `${KORRIGIERT}.mp3`))).toBe(false);
    expect(existsSync(join(app.dataDir, 'results', KORRIGIERT))).toBe(false);
    // its finished job went from the queue with it
    expect(app.json('results/_queue.json').jobs.map((j: { audio: string }) => j.audio)).not.toContain(`${KORRIGIERT}.mp3`);
  });
});
