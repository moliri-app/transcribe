// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, test } from './fixtures';

test.describe('Start und Navigation', () => {
  test('die App-Leiste führt zu allen Seiten und zeigt, wo man ist', async ({ page }) => {
    await page.goto('/');
    const nav = page.getByRole('navigation', { name: 'Hauptnavigation' });
    const heading = (name: string) => page.getByRole('heading', { level: 1, name });
    // Live and the editor have no fixed heading: each shows its recording, so check what they always show
    for (const [link, shown] of [
      ['Live', page.getByRole('searchbox', { name: 'In allen Aufnahmen suchen' })],
      ['Einstellungen', heading('Einstellungen')],
      ['Editor', page.getByRole('combobox', { name: 'Aufnahme wählen' })],
      ['Bibliothek', heading('Bibliothek')],
    ] as const) {
      await nav.getByRole('link', { name: link, exact: true }).click();
      await expect(nav.getByRole('link', { name: link, exact: true })).toHaveAttribute('aria-current', 'page');
      await expect(shown).toBeVisible();
    }
    await expect(nav.getByRole('link', { name: 'Editor', exact: true })).not.toHaveAttribute('aria-current', 'page');
    // „Modelle vergleichen“ is gone
    await expect(nav.getByRole('link', { name: 'Modelle vergleichen' })).toHaveCount(0);
  });

  test('auf dem Handy liegen die übrigen Einträge im Menü „Mehr“, ohne seitliches Scrollen', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/queue');
    const nav = page.getByRole('navigation', { name: 'Hauptnavigation' });
    await expect(nav.getByRole('link', { name: 'Bibliothek', exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    const more = nav.getByRole('button', { name: 'Mehr' });
    await more.click();
    await expect(more).toHaveAttribute('aria-expanded', 'true');
    // Tastatur: der Fokus steht im Menü, Pfeil runter geht zum nächsten Eintrag
    await expect(nav.getByRole('menuitem', { name: 'Live' })).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expect(nav.getByRole('menuitem', { name: 'Einstellungen' })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { level: 1, name: 'Einstellungen' })).toBeAttached();
    await nav.getByRole('button', { name: 'Mehr' }).click();
    await expect(nav.getByRole('menuitem', { name: 'Einstellungen' })).toHaveAttribute('aria-current', 'page');
    await page.keyboard.press('Escape');
    await expect(nav.getByRole('menu')).toHaveCount(0);
    await expect(nav.getByRole('button', { name: 'Mehr' })).toBeFocused();
  });

  test('ist alles eingerichtet, erscheint kein Hinweis zur Ersteinrichtung', async ({ page }) => {
    await page.goto('/editor');
    await expect(page.getByRole('combobox', { name: 'Aufnahme wählen' })).toBeVisible();
    await page.waitForTimeout(1500);
    await expect(page.getByText(/Erste Einrichtung|Download fehlgeschlagen/)).toHaveCount(0);
  });
});

test.describe('Ersteinrichtung, wenn ein Modell fehlt', () => {
  test.use({ appOptions: { missing: ['whisper-base'] } });

  test('ein gescheiterter Download wird gemeldet und kann in den Einstellungen wiederholt werden', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText(/Download fehlgeschlagen \(Spracherkennung \(Whisper base\)\)/)).toBeVisible();
    await page.getByRole('navigation', { name: 'Hauptnavigation' }).getByRole('link', { name: 'Einstellungen' }).click();
    await expect(page.getByRole('button', { name: 'Erneut laden' })).toBeVisible();
    await expect(page.getByText('Spracherkennung (Whisper base)', { exact: true })).toBeVisible();
  });
});
