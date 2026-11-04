// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { annotationsOf, expect, INTERVIEW, openEditor, saveState, test, word } from './fixtures';

test('im Transkript suchen: Trefferzahl, Springen mit Enter und Pfeilen', async ({ page }) => {
  await openEditor(page);
  const search = page.getByRole('searchbox', { name: 'Im Transkript suchen' });
  await search.fill('Papier');
  await expect(page.getByText('1 von 2', { exact: true })).toBeVisible();
  await search.press('Enter');
  await expect(page.getByText('2 von 2', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Nächster Treffer' }).click();
  await expect(page.getByText('1 von 2', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Voriger Treffer' }).click();
  await expect(page.getByText('2 von 2', { exact: true })).toBeVisible();

  await search.fill('Zellstoff');
  await expect(page.getByText('kein Treffer', { exact: true })).toBeVisible();
  await search.press('Escape');
  await expect(search).toHaveValue('');
});

test('Doppelklick wählt nur das Wort: Getipptes verschmilzt nicht mit dem nächsten', async ({ page, app }) => {
  await openEditor(page);
  await word(page, 'Habaga').dblclick();
  await page.keyboard.type('Herr Berger');
  await page.keyboard.press('Enter');
  await expect(word(page, 'Berger')).toBeVisible();
  await expect(word(page, 'Sie')).toBeVisible();
  await expect(saveState(page)).toHaveText(/^gespeichert/);
  expect(annotationsOf(app, INTERVIEW).edits).toEqual([{ start: 2.166, end: 2.74, text: 'Herr Berger' }]);
});
