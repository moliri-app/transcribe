// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { test as base, expect, type Download, type Locator, type Page } from '@playwright/test';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { startApp } from '../app.mjs';

// The contract reaches the UI only through roles, accessible names and visible text, never CSS classes,
// so it runs unchanged against the SvelteKit pages. The data files (_annotations.json, _final.json) are
// read to check what was saved: they belong to the unchanged HTTP API and storage.

export type App = {
  url: string;
  dataDir: string;
  /** the stand-in for speech recognition services (appOptions.api): OpenAI-compatible at <apiUrl>/v1, ElevenLabs at <apiUrl> */
  apiUrl?: string;
  /** reads a JSON file below the data directory, e.g. 'results/Papierfabrik Interview/_annotations.json' */
  json(rel: string): any;
  writeJSON(rel: string, value: unknown): void;
};

type Options = { appOptions: { missing?: string[]; api?: boolean } };

export const test = base.extend<Options & { app: App }>({
  appOptions: [{}, { option: true }],
  app: async ({ appOptions }, use) => {
    const a = await startApp(appOptions);
    try {
      await use({
        url: a.url,
        dataDir: a.dataDir,
        apiUrl: a.apiUrl,
        json: rel => JSON.parse(readFileSync(join(a.dataDir, rel), 'utf8')),
        writeJSON: (rel, v) => writeFileSync(join(a.dataDir, rel), JSON.stringify(v, null, 1)),
      });
    } finally {
      await a.stop();
    }
  },
  baseURL: async ({ app }, use) => use(app.url),
});
export { expect };

// ---- test data (testdata/) ----
// "Papierfabrik Interview": machine result only. Speakers „Sprecher 1“ (questions) and „Sprecher 2“ (answers), S1 and S2 in the data;
// the recognizer heard "Habaga" for "Herr Berger".
// "Papierfabrik mit Korrekturen": the same recording with prepared corrections: speakers named
// Interviewerin / Herr Berger, "Habaga" replaced by "Herr Berger", clips "Anfang als Lehrling"
// (Ausbildung) and "Filze waschen" (Arbeit, Ausbildung).
// "Neue Aufnahme": not transcribed yet.
export const INTERVIEW = 'Papierfabrik Interview';
export const KORRIGIERT = 'Papierfabrik mit Korrekturen';
const FIRST_SPEAKER: Record<string, string> = { [INTERVIEW]: 'Sprecher 1', [KORRIGIERT]: 'Interviewerin' };
export const annotationsOf = (app: App, stem: string) => app.json(`results/${stem}/_annotations.json`);

// ---- editor ----
export const transcript = (page: Page) => page.getByRole('region', { name: 'Transkript' });
export const word = (page: Page, text: string) => transcript(page).getByText(text, { exact: true });
export const speakerList = (page: Page) => page.getByRole('list', { name: 'Sprecher' });
export const clipList = (page: Page) => page.getByRole('list', { name: 'Clips' });
export const corrections = (page: Page) => page.getByRole('list', { name: 'Korrekturen' });
export const dialog = (page: Page) => page.getByRole('dialog');
/** picks a language in a dialog: a search field whose suggestions start with the name */
export async function chooseLanguage(page: Page, name: string) {
  const box = dialog(page).getByRole('combobox', { name: 'Sprache' });
  await box.fill(name);
  await dialog(page).getByRole('option', { name: new RegExp(`^${name}(\\s|$)`) }).first().click();
}
export const saveState = (page: Page) => page.getByRole('status');
/** the word being spoken while the recording plays */
export const spoken = (page: Page) => transcript(page).locator('[aria-current="true"]');
export const playButton = (page: Page) => page.getByRole('button', { name: /^(Abspielen|Anhalten)/ });

export async function openEditor(page: Page, recording = INTERVIEW) {
  await listenSpoken(page);
  await page.goto('/editor');
  const sel = page.getByRole('combobox', { name: 'Aufnahme wählen' });
  await expect(sel.getByRole('option', { name: recording, exact: true })).toBeAttached();
  await sel.selectOption({ label: recording });
  await expect(transcript(page).getByRole('button', { name: FIRST_SPEAKER[recording], exact: true }).first()).toBeVisible();
  await expect(word(page, 'Gespräch.')).toBeVisible();
}

// Which words were marked as spoken, in order, with the page's clock. Recorded in the page as the marks
// change (aria-current="true"), because polling from the test misses short words when the machine is busy.
type Heard = { w: string; t: number };
async function listenSpoken(page: Page) {
  await page.addInitScript(() => {
    const log: { w: string; t: number }[] = ((window as any).__heard = []);
    new MutationObserver(ms => {
      for (const m of ms) {
        const el = m.target as Element;
        if (el.getAttribute('aria-current') !== 'true') continue;
        const w = (el.textContent || '').trim();
        if (log.length && log[log.length - 1].w === w && performance.now() - log[log.length - 1].t < 300) continue;
        log.push({ w, t: performance.now() });
      }
    }).observe(document, { subtree: true, attributes: true, attributeFilter: ['aria-current'] });
  });
}
/** words marked as spoken since the page was opened or since forgetHeard */
export const heard = (page: Page): Promise<Heard[]> => page.evaluate(() => (window as any).__heard.slice());
export const heardWords = async (page: Page) => (await heard(page)).map(h => h.w);
export const forgetHeard = (page: Page) => page.evaluate(() => { (window as any).__heard.length = 0; });
/** waits until the word has been marked as spoken */
export async function waitHeard(page: Page, text: string, timeout = 10_000) {
  await expect.poll(() => heardWords(page), { timeout, message: `„${text}“ wird beim Abspielen markiert` }).toContain(text);
}

/** selects text with the mouse, from one word to the end of another */
export async function dragSelect(page: Page, from: Locator, to: Locator) {
  const a = (await from.boundingBox())!, b = (await to.boundingBox())!;
  // from the middle of the first word: its start may carry a time anchor, which drags instead
  await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 8 });
  await page.mouse.move(b.x + b.width - 1, b.y + b.height / 2, { steps: 4 });
  await page.mouse.up();
}

/** selects one word in the text, ready to be typed over */
export async function selectWord(page: Page, text: string) {
  const w = word(page, text);
  await w.click();
  await w.selectText();
}

export async function downloadText(d: Download) {
  return readFileSync((await d.path())!, 'utf8');
}
