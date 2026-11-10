// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, test } from './fixtures';

test.describe('Einstellungen', () => {
  test('Vorgaben werden sofort gespeichert und gelten in der Bibliothek', async ({ page, app }) => {
    await page.goto('/settings');
    // only speakers and languages: a preset fixed nothing but the speakers, threads the app chooses
    await expect(page.getByRole('combobox', { name: 'Voreinstellung' })).toHaveCount(0);
    await expect(page.getByRole('spinbutton', { name: 'Threads' })).toHaveCount(0);
    const sprecher = page.getByRole('combobox', { name: 'Sprecher' });
    await expect(sprecher).toHaveValue('');
    await sprecher.selectOption('1');
    await expect(page.getByRole('status')).toHaveText('Gespeichert.');
    await page.getByRole('switch', { name: 'Sofort mit diesen Vorgaben starten' }).check();
    await expect(page.getByRole('status')).toHaveText('Gespeichert.');
    expect(app.json('settings.json')).toMatchObject({ preset: 'auto', speakers: 1, dropMode: 'auto' });

    await page.reload();
    await expect(sprecher).toHaveValue('1');
    await expect(page.getByRole('switch', { name: 'Sofort mit diesen Vorgaben starten' })).toBeChecked();

    await page.goto('/');
    await expect(page.getByRole('combobox', { name: 'Sprecher' })).toHaveValue('1');
  });

  test('eine früher gewählte Voreinstellung wird zur Sprecherzahl', async ({ page, app }) => {
    app.writeJSON('settings.json', { ...app.json('settings.json'), preset: 'interview', speakers: 0 });
    await page.goto('/settings');
    await expect(page.getByRole('combobox', { name: 'Sprecher' })).toHaveValue('2');
  });

  test('das Menü zeigt eine Kategorie mit ihren Abschnitten und, wo man ist', async ({ page }) => {
    await page.goto('/settings');
    const menu = page.getByRole('navigation', { name: 'Kategorien der Einstellungen' });
    await expect(menu.getByRole('link', { name: 'Transkribieren' })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('heading', { name: 'Erkennung', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Grafikkarte' })).toHaveCount(0);

    await menu.getByRole('link', { name: 'System' }).click();
    await expect(page).toHaveURL(/#system$/);
    await expect(menu.getByRole('link', { name: 'System' })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('heading', { name: 'Erkennung', exact: true })).toHaveCount(0);
    await menu.getByRole('link', { name: 'Speicher und Modelle' }).click();
    await expect(page).toHaveURL(/#speicher$/);
    await expect(page.getByRole('heading', { name: 'Speicher und Modelle' })).toBeInViewport();
    await expect(menu.getByRole('link', { name: 'Speicher und Modelle' })).toHaveAttribute('aria-current', 'location');

    // a section in the address opens its category
    await page.goto('/settings#stimme');
    await expect(menu.getByRole('link', { name: 'Live und Diktieren' })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('region', { name: 'Meine Stimme' })).toBeInViewport();
  });

  test('„Im Hintergrund bereit“ schaltet die Mikrofon-Aufnahme für Programme', async ({ page, app }) => {
    await page.goto('/settings#programme');
    const bereit = page.getByRole('switch', { name: 'Im Hintergrund bereit' });
    await expect(bereit).toBeChecked();
    await bereit.uncheck();
    await expect.poll(() => app.json('settings.json').liveOff).toBe(true);
    await bereit.check();
    await expect.poll(() => app.json('settings.json').liveOff).toBeUndefined();
  });

  test('mehrere Sprachen lassen sich wählen und gelten in der Bibliothek', async ({ page, app }) => {
    await page.goto('/settings');
    const sprachen = page.getByRole('combobox', { name: 'Sprachen' });
    await expect(sprachen).toHaveAttribute('placeholder', 'Automatisch');
    await sprachen.fill('deu');
    // only in the field's own list: the text recognition section has a language select too
    await page.getByRole('listbox', { name: 'Sprachen' }).getByRole('option', { name: /^Deutsch/ }).click();
    await sprachen.fill('en');
    await sprachen.press('Enter');
    await expect(page.getByRole('status')).toHaveText('Gespeichert.');
    await expect.poll(() => app.json('settings.json').language).toBe('de,en');

    await page.goto('/');
    await expect(page.getByRole('button', { name: 'Deutsch entfernen' })).toBeVisible();
    await page.getByRole('button', { name: 'Englisch entfernen' }).click();
    await expect(page.getByRole('button', { name: 'Englisch entfernen' })).toHaveCount(0);
    await page.getByRole('combobox', { name: 'Sprachen' }).press('Backspace');
    await expect(page.getByRole('combobox', { name: 'Sprachen' })).toHaveAttribute('placeholder', 'Automatisch');
  });

  test('Diktieren: Tastenkürzel und Ziel des Textes werden gespeichert', async ({ page, app }) => {
    test.skip(process.platform !== 'win32', 'das Tastenkürzel gibt es nur unter Windows');
    await page.goto('/settings#diktieren');
    const bereich = page.getByRole('region', { name: 'Diktieren in jedem Programm' });
    // the tests start the app without its tray icon, which receives the shortcut
    await expect(bereich).toContainText('nur, solange Transcribe mit seinem Symbol im Infobereich läuft');
    const kuerzel = bereich.getByRole('combobox', { name: 'Tastenkürzel' });
    await expect(kuerzel).toHaveValue('ctrl+alt+space');
    await expect(bereich.locator('kbd').first()).toHaveText('Strg + Alt + Leertaste');
    await kuerzel.selectOption('ctrl+alt+d');
    await expect(bereich.locator('kbd').first()).toHaveText('Strg + Alt + D');
    const einfuegen = bereich.getByRole('switch', { name: 'Am Cursor einfügen' });
    const ablage = bereich.getByRole('switch', { name: 'In die Zwischenablage' });
    await expect(einfuegen).toBeChecked();
    await expect(ablage).toBeChecked();
    await einfuegen.uncheck();
    await expect(bereich).toContainText('Der Text wird nirgends eingefügt');
    await ablage.uncheck();
    await expect(bereich).toContainText('Das Diktat erscheint nur im kleinen Fenster');
    expect(app.json('settings.json')).toMatchObject({ dictateKey: 'ctrl+alt+d', dictateNoInsert: true, dictateNoClipboard: true });

    await page.reload();
    await expect(kuerzel).toHaveValue('ctrl+alt+d');
    await expect(einfuegen).not.toBeChecked();
    await expect(ablage).not.toBeChecked();
    await ablage.check();
    await expect(bereich).not.toContainText('Das Diktat erscheint nur im kleinen Fenster');
    await kuerzel.selectOption('off');
    await expect(einfuegen).toBeDisabled();
    await expect(ablage).toBeDisabled();
    await expect(bereich).not.toContainText('Symbol im Infobereich läuft');
  });

  test('zeigt Datenordner, Modelle und Programm', async ({ page, app }) => {
    await page.goto('/settings#system');
    await expect(page.getByRole('heading', { name: 'Speicherorte' })).toBeVisible();
    await expect(page.getByRole('list', { name: 'Speicherorte' }).getByText(app.dataDir, { exact: true })).toBeVisible();
    // started with --data: the place is fixed, and cannot move here
    await expect(page.getByRole('heading', { name: 'Ort des Datenordners' })).toBeVisible();
    await expect(page.getByText('Beim Start mit')).toContainText('festgelegt');
    await expect(page.getByRole('button', { name: 'Anderen Ort wählen …' })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Modelle und Komponenten' })).toBeVisible();
    for (const label of ['Erkennungs-Runtime (sherpa-onnx)', 'Spracherkennung (Parakeet v3)', 'Sprechertrennung (pyannote)']) {
      await expect(page.getByText(label, { exact: true })).toBeVisible();
    }
    await expect(page.getByText('installiert', { exact: true }).first()).toBeVisible();
    // Whisper turbo, Kroko (German live) and the PP-OCRv5 models are only fetched when needed; row by row,
    // since which rows there are depends on the platform (Tesseract) and on the graphics card (CUDA)
    const liste = page.getByRole('list', { name: 'Modelle und Komponenten' });
    for (const label of ['Fremdsprachen (Whisper turbo)', 'Deutsch live (Kroko)', 'Texterkennung (PP-OCRv5)']) {
      await expect(liste.getByRole('listitem').filter({ hasText: label })).toContainText('bei Bedarf');
    }
    await expect(liste.getByRole('listitem').filter({ hasText: 'Spracherkennung (Parakeet v3)' })).toContainText('installiert');
    await expect(page.getByRole('heading', { name: 'Über Transcribe', exact: true })).toBeVisible();
  });
});
