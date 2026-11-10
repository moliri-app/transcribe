// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { existsSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { MIKROFONE } from '../app.mjs';
import { expect, test } from './fixtures';

// testdata/live holds seven recordings: three started by hand on the Live page (a meeting mixing
// microphone and system audio, notes on a school tour, an English webinar) and four that programs
// asked for over the API (Moliri sent audio or had the app listen, one of them broke off; a web page
// sent a test). The notes on the school tour saved their audio. The test data has no recognition
// models, so nothing is recorded here.
const recordings = (page: import('@playwright/test').Page) =>
  page.getByRole('list', { name: 'Aufnahmen' }).getByRole('listitem');
const text = (page: import('@playwright/test').Page) => page.getByRole('log', { name: 'Text der Aufnahme' });
const TOUR = '20261027-143002-4';

test.describe('Live', () => {
  test('lists all recordings, filtered by where they came from', async ({ page }) => {
    await page.goto('/live');
    await expect(recordings(page)).toHaveCount(7);
    // the newest one is shown in full
    await expect(text(page)).toContainText('Die Papierfabrik bekommt den großen Raum im Erdgeschoss');

    await page.getByRole('radio', { name: /^Per API/ }).check({ force: true });
    await expect(recordings(page)).toHaveCount(4);
    await expect(recordings(page).filter({ hasText: 'abgebrochen' })).toHaveCount(1);
    await page.getByRole('radio', { name: /^Von Hand/ }).check({ force: true });
    await expect(recordings(page)).toHaveCount(3);

    await page.getByRole('combobox', { name: 'Sortierung' }).selectOption('alt');
    await expect(recordings(page).first()).toContainText('Welcome everyone to this webinar');
  });

  test('opened afresh with other parameters in the address, it shows the recordings and keeps them', async ({ page }) => {
    const fehler: string[] = [];
    page.on('pageerror', (e) => fehler.push(e.message));
    await page.goto('/live?neu');
    await expect(recordings(page)).toHaveCount(7);
    await expect(text(page)).toContainText('Die Papierfabrik bekommt den großen Raum im Erdgeschoss');
    // the newest recording goes into the address, "neu" stays as it was
    await expect(page).toHaveURL(/\/live\?neu&aufnahme=/);

    await page.goto(`/live?neu&x=1&aufnahme=${TOUR}`);
    await expect(text(page)).toContainText('Notizen zur Führung mit der Grundschule');
    await expect(page).toHaveURL(new RegExp(`/live\\?neu&x=1&aufnahme=${TOUR}$`));
    expect(fehler).toEqual([]);
  });

  test('searches all recordings loosely and shows the best hit with its sentence marked', async ({ page }) => {
    await page.goto('/live');
    const search = page.getByRole('searchbox', { name: 'In allen Aufnahmen suchen' });
    // a letter missing still finds "Absperrung"
    await search.fill('absperung');
    await expect(recordings(page)).toHaveCount(1);
    await expect(text(page)).toContainText('Beim Holländer brauchen wir eine Absperrung');
    await expect(text(page).locator('mark').first()).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`aufnahme=${TOUR}`));

    // the name of a program counts too
    await search.fill('moliri');
    await expect(recordings(page)).toHaveCount(3);

    await search.press('Escape');
    await expect(search).toHaveValue('');
    await expect(recordings(page)).toHaveCount(7);
  });

  test('plays the saved file of a recording, marking the sentence that plays', async ({ page }) => {
    await page.goto(`/live?aufnahme=${TOUR}`);
    await expect(page.getByRole('button', { name: 'Abspielen' })).toBeVisible();
    // a sentence's time plays from there
    await page.getByRole('button', { name: '0:11' }).click();
    await expect(page.getByRole('button', { name: 'Anhalten' })).toBeVisible();
    await expect(text(page).locator('.satz.spielt')).toContainText('Beim Holländer brauchen wir eine Absperrung');
    await page.getByRole('button', { name: 'Anhalten' }).click();
    expect(await page.locator('audio').evaluate((a: HTMLAudioElement) => a.currentTime)).toBeGreaterThan(10.5);
  });

  test('"Im Hintergrund bereit" switches off and on; while off, programs may not have it record and recording by hand still works', async ({ page, app }) => {
    await page.goto('/live');
    const mode = page.getByRole('switch', { name: 'Im Hintergrund bereit' });
    await expect(mode).toHaveAttribute('aria-checked', 'true');
    await mode.click();
    await expect(mode).toHaveAttribute('aria-checked', 'false');
    await expect(page.getByRole('button', { name: /Aufnahme starten/ })).toBeEnabled();
    // the switch turns at once, the setting is saved a moment later
    await expect.poll(() => app.json('settings.json').liveOff).toBe(true);

    const refused = await page.evaluate(
      () =>
        new Promise<string>((resolve) => {
          const ws = new WebSocket(`ws://${location.host}/api/live?mode=mic&name=Test`);
          ws.onmessage = (e) => resolve(JSON.parse(e.data).message);
        })
    );
    expect(refused).toContain('„Im Hintergrund bereit“ ist aus');

    await mode.click();
    await expect(mode).toHaveAttribute('aria-checked', 'true');
    await expect.poll(() => app.json('settings.json').liveOff).toBeUndefined();
  });

  test('deletes a recording', async ({ page, app }) => {
    await page.goto(`/live?aufnahme=${TOUR}`);
    await expect(text(page)).toContainText('Notizen zur Führung mit der Grundschule');
    await page.getByRole('button', { name: 'Löschen' }).click();
    await expect(recordings(page)).toHaveCount(6);
    expect(existsSync(join(app.dataDir, 'live', `${TOUR}.json`))).toBe(false);
  });

  test('the microphones follow what is plugged in; the system default is a choice of its own', async ({ page, app }) => {
    const plug = (...names: string[]) => writeFileSync(join(app.dataDir, MIKROFONE), names.join('\n'));
    plug('* Mikrofonarray (Realtek Audio)');
    await page.goto('/live');
    await page.getByRole('button', { name: 'Einrichten' }).click();
    const mic = page.getByRole('dialog', { name: 'Aufnahme einrichten' }).getByRole('combobox', { name: 'Mikrofon' });
    const options = mic.getByRole('option');
    await expect(options).toHaveText(['Systemstandard: Mikrofonarray (Realtek Audio)', 'Mikrofonarray (Realtek Audio)']);
    await expect(mic).toHaveValue('');

    // a headset is plugged in and Windows makes it the default: the list changes without reloading
    plug('Mikrofonarray (Realtek Audio)', '* Headset (Jabra Evolve2)');
    await expect(options).toHaveText(['Systemstandard: Headset (Jabra Evolve2)', 'Mikrofonarray (Realtek Audio)', 'Headset (Jabra Evolve2)']);

    // chosen by name, it stays chosen while unplugged
    await mic.selectOption('Headset (Jabra Evolve2)');
    plug('* Mikrofonarray (Realtek Audio)');
    await expect(options).toHaveText(['Systemstandard: Mikrofonarray (Realtek Audio)', 'Headset (Jabra Evolve2) (nicht angeschlossen)', 'Mikrofonarray (Realtek Audio)']);
    await expect(mic).toHaveValue('Headset (Jabra Evolve2)');
    await mic.selectOption('');
    await expect(options).toHaveCount(2);
  });

  test('recording settings sit in their own window; connecting programs leads to the settings', async ({ page }) => {
    await page.goto('/live');
    await page.getByRole('button', { name: 'Einrichten' }).click();
    const settings = page.getByRole('dialog', { name: 'Aufnahme einrichten' });
    await expect(settings.getByRole('combobox', { name: 'Mikrofon' })).toBeVisible();
    await settings.getByRole('radio', { name: /^Systemaudio/ }).check();
    await expect(settings.getByRole('combobox', { name: 'Mikrofon' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Aufnahme starten/ })).toContainText('Systemaudio');
    await page.keyboard.press('Escape');
    await expect(settings).toHaveCount(0);

    await page.getByRole('link', { name: 'Programme verbinden' }).click();
    await expect(page).toHaveURL(/\/settings#programme$/);
    await expect(page.getByRole('heading', { name: 'Zugriff' })).toBeVisible();
    await expect(page.getByText(/^ws:\/\/.+\/api\/live$/).first()).toBeVisible();
    // the view it had before: old links lead there too
    await page.goto('/live?verbinden');
    await expect(page).toHaveURL(/\/settings#programme$/);
  });

  test('a page that may not use Transcribe is asked about wherever one is; allowed, it may', async ({ page, app }) => {
    await page.goto('/');
    const refused = await page.request.get('/api/ocr/engines', { headers: { Origin: 'https://moliri.example.org' } });
    expect(refused.status()).toBe(403);
    const frage = page.getByRole('region', { name: 'Darf moliri.example.org Transcribe nutzen?' });
    await expect(frage).toBeVisible();
    await frage.getByRole('button', { name: 'Erlauben' }).click();
    await expect(page.getByRole('heading', { name: 'moliri.example.org darf Transcribe nutzen' })).toBeVisible();
    await expect.poll(() => app.json('settings.json').liveOrigins).toEqual(['https://moliri.example.org']);
    expect((await page.request.get('/api/ocr/engines', { headers: { Origin: 'https://moliri.example.org' } })).status()).toBe(200);

    // turned down, it is asked about again the next time, and stays out
    await page.request.get('/api/ocr/engines', { headers: { Origin: 'https://other.example.org' } });
    await page.goto('/settings#zugriff');
    const andere = page.getByRole('region', { name: 'Darf other.example.org Transcribe nutzen?' });
    await expect(page.getByRole('list', { name: 'Möchten Transcribe nutzen' })).toContainText('https://other.example.org');
    await andere.getByRole('button', { name: 'Ablehnen' }).click();
    await expect(andere).toHaveCount(0);
    await expect(page.getByRole('list', { name: 'Möchten Transcribe nutzen' })).toHaveCount(0);
    await expect(page.getByRole('list', { name: 'Erlaubte Webseiten' })).toContainText('https://moliri.example.org');
  });

  test('my voice: own words as chips and spoken punctuation, kept in the settings', async ({ page }) => {
    await page.goto('/live');
    await page.getByRole('button', { name: 'Einrichten' }).click();
    const settings = page.getByRole('dialog', { name: 'Aufnahme einrichten' });
    await expect(settings.getByRole('combobox', { name: 'Sprachen' })).toBeVisible();
    await settings.getByRole('link', { name: /alle Einstellungen für Live/ }).click();
    const stimme = page.getByRole('region', { name: 'Meine Stimme' });
    await expect(stimme.getByRole('combobox', { name: 'Sprachen' })).toBeVisible();
    const feld = stimme.getByRole('textbox', { name: 'Meine Wörter' });
    await feld.fill('Moliri');
    await feld.press('Enter');
    await feld.fill('Obeya');
    await feld.press('Enter');
    await feld.fill('moliri'); // there already
    await feld.press('Enter');
    await expect(stimme.getByRole('button', { name: /entfernen$/ })).toHaveCount(2);
    await stimme.getByRole('button', { name: 'Moliri entfernen' }).click();
    const ansagen = stimme.getByRole('switch', { name: /Satzzeichen ansagen/ });
    await expect(ansagen).toHaveAttribute('aria-checked', 'true');
    await ansagen.click();

    await page.reload();
    await expect(stimme.getByRole('button', { name: /entfernen$/ })).toHaveCount(1);
    await expect(stimme.getByRole('button', { name: 'Obeya entfernen' })).toBeVisible();
    await expect(ansagen).toHaveAttribute('aria-checked', 'false');
    await ansagen.click();
    await stimme.getByRole('button', { name: 'Obeya entfernen' }).click();
  });
  test('what programs ask of the language model and search by meaning is listed with request, answer and model', async ({ page }) => {
    // the test data has neither model: both requests are refused, and listed with why
    const frage = await page.request.post('/api/filter?name=Moliri', {
      data: { text: 'Gemälde aus dem 19. Jahrhundert', catalog: 'titel: Titel (Text)\njahr: Datierung (Zahl)', fields: ['titel', 'jahr'] },
    });
    expect(frage.status()).toBe(503);
    const suche = await page.request.post('/api/embed?name=Moliri', { data: { texts: ['Zinnteller', 'Gravur'], kind: 'document' } });
    expect(suche.status()).toBe(503);

    await page.goto('/live');
    await page.getByRole('radio', { name: /^KI & OCR/ }).check({ force: true });
    await expect(recordings(page)).toHaveCount(2);
    await expect(page.getByRole('status').filter({ hasText: '2 Anfragen' })).toBeVisible();
    await recordings(page).filter({ hasText: 'Gemälde aus dem 19. Jahrhundert' }).getByRole('button').click();

    const anfrage = page.getByRole('region', { name: 'Anfrage und Antwort' });
    await expect(page.getByRole('heading', { level: 1 })).toContainText('Filter in eigenen Worten');
    await expect(page.getByText('Qwen 2.5 7B Instruct').first()).toBeVisible();
    await expect(anfrage.getByRole('alert')).toHaveText('Das Sprachmodell ist noch nicht heruntergeladen');
    // the request holds the question and the prompt the model would have read
    await expect(anfrage).toContainText('Frage: Gemälde aus dem 19. Jahrhundert');
    await expect(anfrage).toContainText('jahr: Datierung (Zahl)');

    await recordings(page).filter({ hasText: 'Zinnteller' }).getByRole('button').click();
    await expect(page.getByRole('heading', { level: 1 })).toContainText('Suche nach Bedeutung');
    await expect(anfrage).toContainText('2. Gravur');
    await expect(anfrage.getByText('2, zusammen 16 Zeichen')).toBeVisible();
  });
});
