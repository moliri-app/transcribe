// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, INTERVIEW, test, word } from './fixtures';

const jobs = (page: import('@playwright/test').Page) => page.getByRole('list', { name: 'Aufträge' }).getByRole('listitem');
// exact: „Texterkennung“ (OCR) is a region of its own, with its own AI service, address and key
const erkennung = (page: import('@playwright/test').Page) => page.getByRole('region', { name: 'Erkennung', exact: true });

// The API is the stand-in from internal/sttapi/sttapitest: it answers every recording with the
// interview's transcript, in which it hears "Herr Berger" where the local models heard "Habaga".
test.use({ appOptions: { api: true } });

test.describe('Erkennung', () => {
  // settings, queue, a job and the editor in one go
  test.setTimeout(120_000);

  test('beides hinterlegt: ein eigener Server als API, und je Auftrag lässt sich wählen, was erkennt', async ({ page, app }) => {
    await page.goto('/settings');
    await expect(page.getByRole('radio', { name: /Nur auf diesem Rechner/ })).toBeChecked();
    await expect(erkennung(page).getByRole('combobox', { name: 'Dienst' })).toHaveCount(0);

    await page.getByRole('radio', { name: /^Beides/ }).check();
    const dienst = erkennung(page).getByRole('combobox', { name: 'Dienst' });
    await expect(dienst).toHaveValue('openai');
    await expect(erkennung(page).getByRole('textbox', { name: 'Adresse' })).toHaveValue('https://api.openai.com/v1');
    await expect(erkennung(page).getByText('Ohne Schlüssel kann die App OpenAI nicht nutzen.')).toBeVisible();
    // ElevenLabs is one service among others
    await expect(dienst.getByRole('option')).toHaveText(['OpenAI', 'Groq', 'Mistral (Voxtral)', 'ElevenLabs', 'Eigener Server (OpenAI-kompatibel)']);
    await dienst.selectOption({ label: 'Eigener Server (OpenAI-kompatibel)' });
    await expect(erkennung(page).getByText('Ohne Schlüssel kann die App')).toHaveCount(0);
    const adresse = erkennung(page).getByRole('textbox', { name: 'Adresse' });
    await expect(adresse).toHaveValue('http://localhost:8000/v1');
    await adresse.fill(`${app.apiUrl}/v1`);
    await adresse.press('Enter');
    await page.getByRole('combobox', { name: 'Live und Diktieren' }).selectOption({ label: 'Über die API' });
    await page.getByRole('combobox', { name: 'Ganze Aufnahmen' }).selectOption({ label: 'Auf diesem Rechner' });
    await expect.poll(() => app.json('settings.json')).toMatchObject({
      recognition: 'both', liveEngine: 'api', fileEngine: 'local', api: { provider: 'custom', url: `${app.apiUrl}/v1` },
    });
    await page.reload();
    await expect(page.getByRole('radio', { name: /^Beides/ })).toBeChecked();
    await expect(erkennung(page).getByRole('combobox', { name: 'Dienst' })).toHaveValue('custom');
    await expect(erkennung(page).getByRole('textbox', { name: 'Adresse' })).toHaveValue(`${app.apiUrl}/v1`);

    await page.goto('/');
    const engine = page.getByRole('combobox', { name: 'Erkennung' });
    await expect(engine).toHaveValue('local');
    // there is no quality, preset or thread count to choose
    await expect(page.getByRole('combobox', { name: 'Qualität' })).toHaveCount(0);
    await expect(page.getByRole('spinbutton', { name: 'Threads' })).toHaveCount(0);
    await engine.selectOption({ label: 'Über die API' });
    await page.getByRole('combobox', { name: 'Sprecher' }).selectOption('2');
    // the interview is transcribed already: its sheet in the library transcribes it anew
    await page.getByRole('button', { name: `${INTERVIEW} neu transkribieren` }).click();

    const job = jobs(page).filter({ hasText: 'über die API' });
    await expect(job).toContainText('2 Sprecher · über die API');
    // once done, it leaves „In Arbeit“; the sheet holds the new transcript
    await expect(job).toHaveCount(0, { timeout: 20_000 });
    expect(app.json(`results/${INTERVIEW}/_machine.json`)).toMatchObject({ engine_used: 'api-custom~whisper-1' });
    await page.getByRole('link', { name: INTERVIEW, exact: true }).click();
    await expect(word(page, 'Berger.')).toBeVisible();
    await expect(word(page, 'Habaga')).toHaveCount(0);
  });

  test('nur über die API: mit ElevenLabs, ohne Schlüssel geht nichts, mit Schlüssel gehen die Aufträge dorthin', async ({ page, app }) => {
    await page.goto('/settings');
    await page.getByRole('radio', { name: /Nur über eine API/ }).check();
    await expect(page.getByRole('combobox', { name: 'Ganze Aufnahmen' })).toHaveCount(0);
    // the models are in the category System
    await page.getByRole('link', { name: 'System' }).click();
    await expect(page.getByText(/deshalb lädt die App keine Modelle/)).toBeVisible();
    await page.getByRole('link', { name: 'Transkribieren' }).click();
    await erkennung(page).getByRole('combobox', { name: 'Dienst' }).selectOption({ label: 'ElevenLabs' });
    const adresse = erkennung(page).getByRole('textbox', { name: 'Adresse' });
    await expect(adresse).toHaveValue('https://api.elevenlabs.io');
    await adresse.fill(app.apiUrl!);
    await adresse.press('Enter');
    await expect.poll(() => app.json('settings.json').api).toMatchObject({ provider: 'elevenlabs', url: app.apiUrl });

    // without a key, nothing can be transcribed
    await page.goto('/');
    await expect(page.getByRole('combobox', { name: 'Erkennung' })).toHaveCount(0);
    await page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3/ }).check();
    await page.getByRole('button', { name: 'Transkribieren', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('noch kein Schlüssel');

    await page.goto('/settings');
    const schluessel = erkennung(page).getByLabel('API-Schlüssel');
    await schluessel.fill('sk_test_0123456789wxyz');
    await schluessel.press('Enter');
    await expect(erkennung(page).getByRole('status').filter({ hasText: 'Schlüssel gespeichert.' })).toBeVisible();
    await expect(schluessel).toHaveAttribute('placeholder', 'hinterlegt (endet auf wxyz)');
    expect(await page.content()).not.toContain('sk_test');
    await page.goto('/');
    await page.getByRole('checkbox', { name: /^Neue Aufnahme\.mp3/ }).check();
    await page.getByRole('button', { name: 'Transkribieren', exact: true }).click();
    const job = jobs(page).filter({ hasText: 'Neue Aufnahme' });
    await expect(job).toContainText('über die API');
    // done: it stands among the transcribed recordings
    await expect(page.getByRole('heading', { name: 'Neue Aufnahme', exact: true })).toBeVisible({ timeout: 20_000 });
  });
});
