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

  test('eine Aufnahme benennen und verschlagworten; die Schlagworte stehen links als Bereiche', async ({ page, app }) => {
    await page.goto('/');
    await card(page, KORRIGIERT).getByRole('button', { name: `Name und Schlagworte von ${KORRIGIERT}` }).click();
    const dlg = page.getByRole('dialog', { name: 'Name und Schlagworte' });
    await expect(dlg).toContainText(`Die Datei heißt weiter „${KORRIGIERT}“`);
    await dlg.getByRole('textbox', { name: 'Name' }).fill('Herr Berger erzählt');
    await dlg.getByRole('textbox', { name: 'Schlagworte' }).fill('Zeitzeugen, Papierfabrik');
    await dlg.getByRole('button', { name: 'Speichern' }).click();
    await expect(dlg).toBeHidden();

    const berger = card(page, 'Herr Berger erzählt');
    await expect(berger).toHaveCount(1);
    await expect(berger.getByRole('button', { name: 'Zeitzeugen' })).toBeVisible();
    expect(app.json('library.json').recordings[KORRIGIERT]).toEqual({ title: 'Herr Berger erzählt', tags: ['Zeitzeugen', 'Papierfabrik'] });

    // a tag is an area of the library, with only its recordings
    const areas = page.getByRole('navigation', { name: 'Bereiche der Bibliothek' });
    await areas.getByRole('button', { name: /^Zeitzeugen/ }).click();
    await expect(page.getByRole('heading', { level: 2, name: 'Zeitzeugen' })).toBeVisible();
    await expect(shelf(page)).toHaveCount(1);

    // the editor names it so, with its tags beside the title
    await berger.getByRole('link').click();
    await expect(page.getByRole('combobox', { name: 'Aufnahme wählen' }).getByRole('option', { name: 'Herr Berger erzählt', exact: true }))
      .toHaveJSProperty('selected', true);
    await expect(page.getByRole('button', { name: 'Zeitzeugen' })).toBeVisible();
  });

  test('ein langer Name mit Schlagworten bleibt in jedem Fenster lesbar, nie Buchstabe für Buchstabe', async ({ page }) => {
    await page.goto('/');
    await card(page, KORRIGIERT).getByRole('button', { name: `Name und Schlagworte von ${KORRIGIERT}` }).click();
    const dlg = page.getByRole('dialog', { name: 'Name und Schlagworte' });
    await dlg.getByRole('textbox', { name: 'Name' }).fill('Zeitzeuge Berger, Papierfabrik');
    await dlg.getByRole('textbox', { name: 'Schlagworte' }).fill('Zeitzeugen, Papierfabrik');
    await dlg.getByRole('button', { name: 'Speichern' }).click();
    await expect(dlg).toBeHidden();

    const title = card(page, 'Zeitzeuge Berger, Papierfabrik').getByRole('heading');
    // 1200 is where the title once shrank to one letter; 360 is a phone-narrow window
    for (const width of [1440, 1300, 1200, 1100, 1000, 900, 800, 640, 480, 360]) {
      await page.setViewportSize({ width, height: 800 });
      const box = await title.evaluate((h) => ({ width: h.getBoundingClientRect().width, lines: Math.round(h.getBoundingClientRect().height / parseFloat(getComputedStyle(h).lineHeight)) }));
      expect(box.width, `title width at ${width} px`).toBeGreaterThan(150);
      expect(box.lines, `title lines at ${width} px`).toBeLessThanOrEqual(3);
    }
    // and the page never needs scrolling sideways
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
  });

  test('in eine Aufnahme ohne Transkript reinhören: unten erscheint die Abspielleiste', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Neue Aufnahme anhören' }).click();
    const bar = page.getByRole('region', { name: 'Wiedergabe' });
    await expect(bar).toContainText('Neue Aufnahme');
    await expect(page.getByRole('button', { name: 'Neue Aufnahme anhalten' })).toBeVisible();
    await expect.poll(() => bar.locator('audio').evaluate((a: HTMLAudioElement) => a.currentTime)).toBeGreaterThan(0);

    // the same button pauses, another recording takes the bar over
    await page.getByRole('button', { name: 'Neue Aufnahme anhalten' }).click();
    await expect(bar.locator('audio')).toHaveJSProperty('paused', true);
    await card(page, INTERVIEW).getByRole('button', { name: `${INTERVIEW} anhören` }).click();
    await expect(bar).toContainText(INTERVIEW);
    await bar.getByRole('button', { name: 'Wiedergabe schließen' }).click();
    await expect(bar).toHaveCount(0);
  });

  test('die Details einer Aufnahme: Dateiname, Ort, Größe, Datum und Transkript', async ({ page, app }) => {
    await page.goto('/');
    // a recording without a transcript shows its size beside the date
    await expect(page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3 58 KB/ })).toBeVisible();
    await card(page, KORRIGIERT).getByRole('button', { name: `Details zu ${KORRIGIERT}` }).click();
    const dlg = page.getByRole('dialog', { name: KORRIGIERT });
    await expect(dlg).toContainText(`${KORRIGIERT}.mp3`);
    await expect(dlg).toContainText(join(app.dataDir, 'audio'));
    await expect(dlg).toContainText('211 KB');
    await expect(dlg).toContainText('0:35');
    await expect(dlg).toContainText('Geändert');
    await expect(dlg).toContainText('1 Korrektur von Hand · 2 Clips');
    await expect(dlg.getByRole('button', { name: 'Abspielen' })).toBeVisible();
    await dlg.getByRole('button', { name: 'Schließen' }).click();
    await expect(dlg).toBeHidden();
  });

  test('eine Aufnahme ausblenden: sie steht nur noch unter „Ausgeblendet“, gelöscht ist nichts', async ({ page, app }) => {
    await page.goto('/');
    await expect(shelf(page)).toHaveCount(2);
    await card(page, KORRIGIERT).getByRole('button', { name: `Name und Schlagworte von ${KORRIGIERT}` }).click();
    await page.getByRole('dialog', { name: 'Name und Schlagworte' }).getByRole('button', { name: 'Ausblenden' }).click();
    await expect(page.getByRole('status')).toContainText(`„${KORRIGIERT}“ ist ausgeblendet.`);
    await expect(shelf(page)).toHaveCount(1);
    expect(existsSync(join(app.dataDir, 'results', KORRIGIERT, '_final.json'))).toBe(true);

    // undo right away
    await page.getByRole('status').getByRole('button', { name: 'Rückgängig' }).click();
    await expect(shelf(page)).toHaveCount(2);

    await card(page, KORRIGIERT).getByRole('button', { name: `Name und Schlagworte von ${KORRIGIERT}` }).click();
    await page.getByRole('dialog', { name: 'Name und Schlagworte' }).getByRole('button', { name: 'Ausblenden' }).click();
    await page.getByRole('navigation', { name: 'Bereiche der Bibliothek' }).getByRole('button', { name: /^Ausgeblendet/ }).click();
    await expect(shelf(page)).toHaveCount(1);
    await card(page, KORRIGIERT).getByRole('button', { name: `${KORRIGIERT} wieder einblenden` }).click();
    // nothing hidden any more: back to all recordings
    await expect(page.getByRole('navigation', { name: 'Bereiche der Bibliothek' }).getByRole('button', { name: /^Ausgeblendet/ })).toHaveCount(0);
    await expect(shelf(page)).toHaveCount(2);
  });
});
