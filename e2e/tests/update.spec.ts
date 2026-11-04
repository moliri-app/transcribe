// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

import { expect, test } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { ROOT, startApp } from '../app.mjs';

// The update end to end: an EXE of version 0.9.0 finds 1.0.0 at a stand-in for GitHub's latest
// release, downloads it on a click, puts it in its own place and comes back as 1.0.0 on the same
// address. Two EXEs are built for it, with the versions set as the release workflow sets them.
test.skip(!!process.env.TRANSCRIBE_EXE || process.platform !== 'win32', 'builds its own EXEs; updates are Windows-only');
test.setTimeout(240_000);

function build(out: string, version: string) {
  const r = spawnSync('go', ['build', '-ldflags', `-X main.version=${version}`, '-o', out, './cmd/transcribe'], {
    cwd: ROOT, stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0' },
  });
  if (r.status !== 0) throw new Error('go build ./cmd/transcribe ist fehlgeschlagen');
}

test('eine ältere Version aktualisiert sich auf Klick auf die neueste', async ({ page }) => {
  const dir = mkdtempSync(join(tmpdir(), 'tx-update-'));
  const exe = join(dir, 'transcribe.exe');
  const next = join(dir, 'release', 'transcribe.exe');
  build(exe, 'v0.9.0');
  build(next, 'v1.0.0');
  const body = readFileSync(next);

  const github: Server = createServer((req, res) => {
    const base = `http://127.0.0.1:${(github.address() as { port: number }).port}`;
    if (req.url === '/latest') {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({
        tag_name: 'v1.0.0', html_url: base + '/page', draft: false, prerelease: false,
        assets: [{ name: 'transcribe.exe', browser_download_url: base + '/transcribe.exe', size: body.length }],
      }));
    } else if (req.url === '/transcribe.exe') {
      res.setHeader('Content-Length', body.length);
      res.end(body);
    } else {
      res.statusCode = 404;
      res.end();
    }
  });
  await new Promise<void>(r => github.listen(0, '127.0.0.1', r));
  const api = `http://127.0.0.1:${(github.address() as { port: number }).port}/latest`;

  const app = await startApp({ exe, env: { TRANSCRIBE_UPDATE_URL: api } });
  try {
    await page.goto(app.url + '/settings#programm');
    await expect(page.getByText('Version 0.9.0', { exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Version 1.0.0 ist da' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Hauptnavigation' }).getByRole('link', { name: 'Update' })).toBeVisible();

    await page.getByRole('button', { name: 'Jetzt aktualisieren' }).click();
    // the app ends, starts the new EXE on the same address, and the page reloads with it
    await expect(page.getByRole('status').filter({ hasText: 'Aktualisiert von Version 0.9.0 auf 1.0.0.' })).toBeVisible({ timeout: 90_000 });
    await expect(page.getByText('Version 1.0.0', { exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Update' })).toHaveCount(0);

    const version = spawnSync(exe, ['-version'], { encoding: 'utf8' }).stdout.trim();
    expect(version).toBe('v1.0.0');
    // the next start deletes the EXE it replaced
    await expect.poll(() => existsSync(join(dir, 'transcribe.old.exe')), { timeout: 40_000 }).toBe(false);
    // the data stayed as they were
    expect(JSON.parse(readFileSync(join(app.dataDir, 'results', '_queue.json'), 'utf8')).jobs).toHaveLength(2);
  } finally {
    // the new version is no child of this test: its pid is in the data directory
    try {
      const { pid } = JSON.parse(readFileSync(join(app.dataDir, 'instance.json'), 'utf8'));
      spawnSync('taskkill', ['/pid', String(pid), '/T', '/F'], { stdio: 'ignore' });
    } catch {}
    await app.stop();
    github.close();
    rmSync(dir, { recursive: true, force: true, maxRetries: 20, retryDelay: 150 });
  }
});
