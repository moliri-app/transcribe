// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/**
 * Ausgewählte Moliri-Komponenten und -Stile nach `src/lib/moliri/` spiegeln.
 *
 * WARUM SPIEGELN. Transcribe soll aussehen und sich bedienen wie Moliri, ohne ein gemeinsames
 * Paket: Moliri bleibt geschlossen, dieses Repo ist offen. Also liegen Kopien hier, und dieses
 * Skript hält sie gleich. Geändert wird in Moliri, nie hier.
 *
 * DIE KOPIE IST FAST STUMPF. Gleiche relative Anordnung wie unter Moliris `src/` (also
 * `lib/components/ui/Button.svelte` → `src/lib/moliri/components/ui/Button.svelte`), dazu genau
 * drei Änderungen, die REWRITES unten aufzählt:
 *   - ein Kopf mit MPL-Hinweis und Herkunft,
 *   - `$lib/components/…` zeigt auf die Spiegel unter `$lib/moliri/components/…`,
 *   - Paraglide (`$lib/../paraglide/messages` und `…/runtime.js`) wird `$lib/moliri/texte.js`:
 *     die deutschen Texte der gespiegelten Dateien, aus Moliris `messages/de.json` erzeugt, und
 *     `getLocale()`, das immer 'de' sagt. Transcribe ist nur deutsch und braucht kein Paraglide.
 * Zeilenenden werden LF.
 *
 * WO MOLIRI LIEGT. `MOLIRI_SRC` (Pfad zum Checkout von moliri_sveltekit), sonst
 * `../MOLIRI-DB/moliri_sveltekit` neben dem Ordner, in dem dieses Repo liegt (auch aus einem
 * Git-Worktree heraus). Ohne Moliri-Checkout geht weder Spiegeln noch Prüfen; der Build braucht
 * beides nicht, die Spiegel sind eingecheckt.
 *
 *   node scripts/sync-moliri.mjs          spiegeln   (pnpm sync:moliri)
 *   node scripts/sync-moliri.mjs --check  nur prüfen (pnpm sync:check, Exit 1 bei Abweichung)
 */

import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const frontend = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const TARGET = path.join(frontend, 'src', 'lib', 'moliri');

/** Gespiegelte Dateien, relativ zu Moliris `src/`. Eine Zeile mehr nimmt eine Datei dazu. */
const FILES = [
	'lib/components/ui/Button.svelte',
	'lib/components/ui/Tabs.svelte',
	'lib/components/ui/Modal.svelte',
	// von Modal importiert (Blättern zwischen Einträgen)
	'lib/components/ui/Pager.svelte',
	'lib/components/ui/SimpleModal.svelte',
	'lib/components/ui/Dropdown.svelte',
	'lib/components/ui/DropdownItem.svelte',
	'lib/components/ui/Toggle.svelte',
	'lib/components/ui/KeyboardTooltip.svelte',
	'lib/components/ui/LoadingWrapper.svelte',
	// Der Transkript-Editor (Moliri hat ihn aus transcribe-go übernommen und weitergebaut). Ohne
	// moliriStore.js: transcribe-go bringt einen eigenen Speicher mit (src/lib/editor/speicher.js).
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
	// Nicht gespiegelt: confirm/ConfirmDialog (zieht ComboInput mit Svelecte aus einem GitHub-Fork
	// herein) und VidstackPlayer (Video-Player; der Editor spielt Audio mit eigenem Mitlesen).
	'scss/colors.scss',
	'scss/reset.scss',
	'scss/components.scss'
];

const TEXTE = 'texte.js';

const MPL = [
	'This Source Code Form is subject to the terms of the Mozilla Public',
	'License, v. 2.0. If a copy of the MPL was not distributed with this',
	'file, You can obtain one at https://mozilla.org/MPL/2.0/.'
];

function header(file) {
	const origin = [
		`Gespiegelt aus Moliri (src/${file}) von frontend/scripts/sync-moliri.mjs.`,
		'Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri.'
	];
	if (file.endsWith('.svelte')) {
		const block = (lines) => `<!-- ${lines.join('\n     ')} -->`;
		return `${block(MPL)}\n${block(origin)}\n`;
	}
	return [...MPL, '', ...origin].map((l) => (l ? `// ${l}` : '//')).join('\n') + '\n\n';
}

/** Die einzigen Änderungen am Inhalt (außer Kopf und Zeilenenden). */
const REWRITES = [
	[/(['"])\$lib\/\.\.\/paraglide\/(?:messages|runtime)(?:\.js)?\1/g, `$1$lib/moliri/${TEXTE}$1`],
	[/(['"])\$lib\/components\//g, '$1$lib/moliri/components/']
];

const lf = (s) => s.replace(/\r\n/g, '\n');

function moliriSource() {
	const candidates = [];
	if (process.env.MOLIRI_SRC) candidates.push(path.resolve(process.env.MOLIRI_SRC));
	else {
		// Hauptcheckout dieses Repos, auch wenn wir in einem Worktree laufen.
		let repo = path.resolve(frontend, '..');
		try {
			const common = execFileSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], {
				cwd: frontend,
				encoding: 'utf8'
			}).trim();
			repo = path.dirname(common);
		} catch {
			// kein Git: dann eben vom Arbeitsverzeichnis aus
		}
		candidates.push(path.resolve(repo, '..', '..', 'MOLIRI-DB', 'moliri_sveltekit'));
		candidates.push(path.resolve(frontend, '..', '..', '..', 'MOLIRI-DB', 'moliri_sveltekit'));
	}
	const found = candidates.find((c) => existsSync(path.join(c, 'src', 'lib', 'components', 'ui')));
	if (!found) {
		console.error(`Moliri-Checkout nicht gefunden (gesucht: ${candidates.join(', ')}).`);
		console.error('MOLIRI_SRC auf den Ordner von moliri_sveltekit setzen.');
		process.exit(2);
	}
	return found;
}

/** Was unter src/lib/moliri/ stehen soll: Pfad → Inhalt. */
async function expected(source) {
	const out = new Map();
	const keys = new Set();
	/** Aufrufe unter einem Namen, den die Datei auch für etwas anderes nimmt (`const m = this.media`): Texte nur, wo es sie gibt */
	const maybe = new Set();
	for (const file of FILES) {
		let text = lf(await readFile(path.join(source, 'src', file), 'utf8'));
		for (const [from, to] of REWRITES) text = text.replace(from, to);
		// die Texte, die die Datei aufruft, unter dem Namen, unter dem sie sie einbindet (meist m, auch msg)
		for (const [, as] of text.matchAll(/import \* as (\w+) from (['"])\$lib\/moliri\/texte\.js\2/g)) {
			const shadowed = new RegExp(`\\b(?:const|let|var)\\s+${as}\\b`).test(text);
			for (const [, k] of text.matchAll(new RegExp(`\\b${as}\\.(\\w+)\\(`, 'g'))) if (k !== 'getLocale') (shadowed ? maybe : keys).add(k);
		}
		const rest = [...text.matchAll(/(['"])\$lib\/(?!moliri\/)([^'"]+)\1/g)].map((x) => x[2]);
		if (rest.length) throw new Error(`${file} importiert ${rest.join(', ')} aus Moliri; mitspiegeln oder hier umschreiben.`);
		out.set(file.replace(/^lib\//, ''), header(file) + text);
	}
	out.set(TEXTE, await texte(source, keys, maybe));
	return out;
}

async function texte(source, /** @type {Set<string>} */ sure, /** @type {Set<string>} */ maybe) {
	const de = JSON.parse(await readFile(path.join(source, 'messages', 'de.json'), 'utf8'));
	const keys = [...new Set([...sure, ...[...maybe].filter((k) => k in de)])].sort();
	const lines = [
		...MPL.map((l) => `// ${l}`),
		'',
		'// Erzeugt von frontend/scripts/sync-moliri.mjs aus Moliris messages/de.json: die Texte, die die',
		'// gespiegelten Komponenten über `m.<name>()` aufrufen. Ersetzt Paraglide; nicht von Hand ändern.',
		'',
		'/** Sprache der Oberfläche (Paraglides getLocale): Transcribe ist nur deutsch. */',
		"export const getLocale = () => 'de';",
		''
	];
	for (const k of keys) {
		const value = de[k];
		if (typeof value !== 'string') throw new Error(`Text ${k} fehlt in Moliris messages/de.json oder ist keine Zeichenkette.`);
		const params = [...value.matchAll(/\{(\w+)\}/g)].map((x) => x[1]);
		const body = params.length
			? '`' + value.replace(/[`\\$]/g, '\\$&').replace(/\{(\w+)\}/g, '${p.$1}') + '`'
			: JSON.stringify(value);
		lines.push(`export const ${k} = (${params.length ? 'p' : ''}) => ${body};`);
	}
	return lines.join('\n') + '\n';
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

const check = process.argv.includes('--check');
const source = moliriSource();
const want = await expected(source);
const problems = [];

for (const [rel, text] of want) {
	const file = path.join(TARGET, rel);
	const have = existsSync(file) ? lf(await readFile(file, 'utf8')) : null;
	if (have === text) continue;
	if (check) problems.push(`${have === null ? 'fehlt' : 'weicht ab'}: src/lib/moliri/${rel}`);
	else {
		await mkdir(path.dirname(file), { recursive: true });
		await writeFile(file, text, 'utf8');
		console.log(`${have === null ? 'neu' : 'aktualisiert'}: src/lib/moliri/${rel}`);
	}
}
for (const rel of await present()) {
	if (!want.has(rel)) problems.push(`nicht in der Liste (löschen oder in FILES aufnehmen): src/lib/moliri/${rel}`);
}

if (problems.length) {
	console.error(problems.join('\n'));
	if (check) console.error(`\nAbweichung von Moliri (${source}). pnpm sync:moliri gleicht an.`);
	process.exit(1);
}
console.log(check ? `Spiegel stimmen mit Moliri überein (${source}).` : `Gespiegelt aus ${source}.`);
