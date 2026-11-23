// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/**
 * Mirror selected Moliri components, styles and its i18n setup into `src/lib/moliri/`, and carry
 * Moliri's translations of their texts into Transcribe's catalogues.
 *
 * WHY MIRROR. Transcribe is to look and work like Moliri without a shared package: Moliri stays
 * closed, this repository is open. So copies live here and this script keeps them equal. Changes
 * are made in Moliri, never here.
 *
 * THE COPY IS VERBATIM. Same relative layout as under Moliri's `src/` (so
 * `lib/components/ui/Button.svelte` → `src/lib/moliri/components/ui/Button.svelte`), with these
 * changes only:
 *   - a header with the MPL notice and the origin,
 *   - `$lib/components/…` and `$lib/i18n/…` point at the mirrors under `$lib/moliri/…`,
 *   - line endings become LF.
 *
 * TEXTS. Transcribe uses wuchale like Moliri (wuchale.config.js): the mirrors keep Moliri's English
 * source texts, wuchale extracts them into `src/locales/<locale>.po` with everything else, and this
 * script then sets the translation of every entry a mirrored file uses to Moliri's translation of
 * the same text (same msgid and context, from Moliri's `src/locales/<locale>.po`). A text Moliri has
 * not translated keeps whatever Transcribe's catalogue has (English if nothing).
 *
 * WHERE MOLIRI LIVES. `MOLIRI_SRC` (path to a checkout of moliri_sveltekit), otherwise
 * `../MOLIRI-DB/moliri_sveltekit` next to the folder this repository sits in (also from a git
 * worktree). Mirroring and checking need a Moliri checkout; the build needs neither, the mirrors and
 * catalogues are checked in.
 *
 *   node scripts/sync-moliri.mjs          mirror, extract, carry translations over (pnpm sync:moliri)
 *   node scripts/sync-moliri.mjs --check  check only (pnpm sync:check, exit 1 on a difference)
 *   --verbose                             also list the mirrors' texts Moliri has not translated
 */

import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import PO from 'pofile';

const frontend = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const TARGET = path.join(frontend, 'src', 'lib', 'moliri');
const LOCALES_DIR = path.join(frontend, 'src', 'locales');

/** Mirrored files, relative to Moliri's `src/`. One more line takes in one more file. */
const FILES = [
	// The interface language (stored choice, browser languages, English) and which strings wuchale
	// takes as text; wuchale.config.js reads both.
	'lib/i18n/locale.js',
	'lib/i18n/reactiveLocale.js',
	'lib/i18n/heuristic.js',
	'lib/components/ui/Button.svelte',
	'lib/components/ui/Tabs.svelte',
	'lib/components/ui/Modal.svelte',
	// imported by Modal (paging between entries; the back button closes it)
	'lib/components/ui/Pager.svelte',
	'lib/history/closeOnBack.svelte.js',
	'lib/history/overlayHistory.js',
	'lib/components/ui/SimpleModal.svelte',
	'lib/components/ui/Dropdown.svelte',
	'lib/components/ui/DropdownItem.svelte',
	'lib/components/ui/Toggle.svelte',
	'lib/components/ui/KeyboardTooltip.svelte',
	'lib/components/ui/LoadingWrapper.svelte',
	// The transcript editor (Moliri took it over from transcribe-go and built on it). Without
	// moliriStore.js: transcribe-go brings a store of its own (src/lib/editor/speicher.js).
	...[
		'TranscriptEditor.svelte',
		'Zeitleiste.svelte',
		'Manuskript.svelte',
		'Seitenspalte.svelte',
		'Player.svelte',
		'SprecherMenue.svelte',
		'Sprachwahl.svelte',
		'Bereich.svelte',
		'Clip.svelte',
		'editor.svelte.js',
		'model.js',
		'sprachen.js'
	].map((f) => `lib/components/files/transcript/editor/${f}`),
	// Not mirrored: confirm/ConfirmDialog (pulls in ComboInput with Svelecte from a GitHub fork) and
	// VidstackPlayer (video player; the editor plays audio with its own follow-along).
	'scss/colors.scss',
	'scss/reset.scss',
	'scss/components.scss'
];

const MPL = [
	'This Source Code Form is subject to the terms of the Mozilla Public',
	'License, v. 2.0. If a copy of the MPL was not distributed with this',
	'file, You can obtain one at https://mozilla.org/MPL/2.0/.'
];

function header(file) {
	const origin = [
		`Mirrored from Moliri (src/${file}) by frontend/scripts/sync-moliri.mjs.`,
		'Do not change it here: change it in Moliri, then pnpm sync:moliri.'
	];
	if (file.endsWith('.svelte')) {
		const block = (lines) => `<!-- ${lines.join('\n     ')} -->`;
		return `${block(MPL)}\n${block(origin)}\n`;
	}
	return [...MPL, '', ...origin].map((l) => (l ? `// ${l}` : '//')).join('\n') + '\n\n';
}

/** The import paths that change (besides the header and the line endings). */
const REWRITES = [[/(['"])\$lib\/(components|i18n|history)\//g, '$1$lib/moliri/$2/']];

const lf = (s) => s.replace(/\r\n/g, '\n');

/** Where a mirrored file sits, relative to the frontend (as wuchale writes it into `#:` references). */
const mirrorRef = (/** @type {string} */ file) => `src/lib/moliri/${file.replace(/^lib\//, '')}`;

function moliriSource() {
	const candidates = [];
	if (process.env.MOLIRI_SRC) candidates.push(path.resolve(process.env.MOLIRI_SRC));
	else {
		// The main checkout of this repository, also when running in a worktree.
		let repo = path.resolve(frontend, '..');
		try {
			const common = execFileSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], {
				cwd: frontend,
				encoding: 'utf8'
			}).trim();
			repo = path.dirname(common);
		} catch {
			// no git: then from the working directory
		}
		candidates.push(path.resolve(repo, '..', '..', 'MOLIRI-DB', 'moliri_sveltekit'));
		candidates.push(path.resolve(frontend, '..', '..', '..', 'MOLIRI-DB', 'moliri_sveltekit'));
	}
	const found = candidates.find((c) => existsSync(path.join(c, 'src', 'lib', 'components', 'ui')));
	if (!found) {
		console.error(`Moliri checkout not found (looked in: ${candidates.join(', ')}).`);
		console.error('Set MOLIRI_SRC to the moliri_sveltekit folder.');
		process.exit(2);
	}
	return found;
}

/** What is to be under src/lib/moliri/: path → content. */
async function expected(source) {
	const out = new Map();
	for (const file of FILES) {
		let text = lf(await readFile(path.join(source, 'src', file), 'utf8'));
		for (const [from, to] of REWRITES) text = text.replace(from, to);
		const rest = [...text.matchAll(/(['"])\$lib\/(?!moliri\/)([^'"]+)\1/g)].map((x) => x[2]);
		if (rest.length) throw new Error(`${file} imports ${rest.join(', ')} from Moliri; mirror it too or rewrite it here.`);
		out.set(file.replace(/^lib\//, ''), header(file) + text);
	}
	return out;
}

async function present(dir = TARGET, prefix = '') {
	const out = [];
	for (const e of await readdir(dir, { withFileTypes: true }).catch(() => [])) {
		const rel = prefix ? `${prefix}/${e.name}` : e.name;
		if (e.isDirectory()) out.push(...(await present(path.join(dir, e.name), rel)));
		else out.push(rel);
	}
	return out;
}

// ---------------------------------------------------------------------------------------------
// Translations: Moliri's for every text a mirrored file uses

const key = (/** @type {any} */ item) => `${item.msgctxt ?? ''}\x04${item.msgid}\x04${item.msgid_plural ?? ''}`;
const translated = (/** @type {string[]} */ msgstr) => msgstr.length > 0 && msgstr.every((s) => s);

/** @returns {Promise<PO | null>} */
async function loadPo(/** @type {string} */ file) {
	return existsSync(file) ? PO.parse(lf(await readFile(file, 'utf8'))) : null;
}

/**
 * Sets the translations of the mirrored files' texts in Transcribe's catalogues to Moliri's.
 * Returns the differences found (in check mode nothing is written).
 */
async function carryTranslations(/** @type {string} */ source, /** @type {boolean} */ check) {
	const mirrored = new Set(FILES.map(mirrorRef));
	const own = await loadPo(path.join(LOCALES_DIR, 'en.po'));
	if (!own) return ['src/locales/en.po missing: run pnpm exec wuchale first'];
	const locales = (await readdir(LOCALES_DIR)).filter((f) => /^[a-z]{2}\.po$/.test(f)).map((f) => f.slice(0, 2));
	const problems = [];
	const untranslated = new Set();
	for (const locale of locales) {
		if (locale === 'en') continue;
		const file = path.join(LOCALES_DIR, `${locale}.po`);
		const po = await loadPo(file);
		const moliri = await loadPo(path.join(source, 'src', 'locales', `${locale}.po`));
		if (!po || !moliri) continue;
		const theirs = new Map(moliri.items.filter((i) => !i.obsolete && translated(i.msgstr)).map((i) => [key(i), i.msgstr]));
		let changed = 0;
		for (const item of po.items) {
			if (item.obsolete || !item.references.some((r) => mirrored.has(r.replace(/:\d+$/, '')))) continue;
			const str = theirs.get(key(item));
			if (!str) {
				untranslated.add(`${locale}: ${JSON.stringify(item.msgid)}`);
				continue;
			}
			if (JSON.stringify(str) === JSON.stringify(item.msgstr)) continue;
			changed++;
			if (check) problems.push(`src/locales/${locale}.po: ${JSON.stringify(item.msgid)} differs from Moliri`);
			else item.msgstr = [...str];
		}
		if (changed && !check) {
			await writeFile(file, po.toString() + '\n', 'utf8');
			console.log(`${changed} translations from Moliri: src/locales/${locale}.po`);
		}
	}
	if (untranslated.size) {
		const by = Object.groupBy([...untranslated], (u) => u.slice(0, 2));
		const counts = Object.entries(by).map(([l, list]) => `${l} ${list.length}`).join(', ');
		console.warn(`Texts of the mirrors Moliri has not translated, left as in Transcribe's catalogue: ${counts}.`);
		if (process.argv.includes('--verbose')) for (const u of untranslated) console.warn(`  ${u}`);
	}
	return problems;
}

// ---------------------------------------------------------------------------------------------

const check = process.argv.includes('--check');
const source = moliriSource();
const want = await expected(source);
const problems = [];

for (const [rel, text] of want) {
	const file = path.join(TARGET, rel);
	const have = existsSync(file) ? lf(await readFile(file, 'utf8')) : null;
	if (have === text) continue;
	if (check) problems.push(`${have === null ? 'missing' : 'differs'}: src/lib/moliri/${rel}`);
	else {
		await mkdir(path.dirname(file), { recursive: true });
		await writeFile(file, text, 'utf8');
		console.log(`${have === null ? 'new' : 'updated'}: src/lib/moliri/${rel}`);
	}
}
for (const rel of await present()) {
	if (!want.has(rel)) problems.push(`not in the list (delete it or add it to FILES): src/lib/moliri/${rel}`);
}

// The mirrors' texts into the catalogues (wuchale's extraction), then Moliri's translations.
if (!check && !problems.length) {
	execFileSync(process.execPath, [path.join(frontend, 'node_modules', 'wuchale', 'dist', 'cli', 'index.js')], {
		cwd: frontend,
		stdio: 'inherit'
	});
}
if (!problems.length) problems.push(...(await carryTranslations(source, check)));

if (problems.length) {
	console.error(problems.join('\n'));
	if (check) console.error(`\nDiffers from Moliri (${source}). pnpm sync:moliri brings it in line.`);
	process.exit(1);
}
console.log(check ? `Mirrors match Moliri (${source}).` : `Mirrored from ${source}.`);
