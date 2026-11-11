// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/**
 * Mirror selected Moliri components and styles into `src/lib/moliri/`.
 *
 * WHY MIRROR. Transcribe is to look and work like Moliri without a shared package: Moliri stays
 * closed, this repository is open. So copies live here and this script keeps them equal. Changes
 * are made in Moliri, never here.
 *
 * THE COPY IS ALMOST VERBATIM. Same relative layout as under Moliri's `src/` (so
 * `lib/components/ui/Button.svelte` → `src/lib/moliri/components/ui/Button.svelte`), with these
 * changes only:
 *   - a header with the MPL notice and the origin,
 *   - `$lib/components/…` points at the mirrors under `$lib/moliri/components/…`,
 *   - the interface texts become German. Transcribe is German only and has no i18n library:
 *     - wuchale (Moliri since its switch, docs/wuchale-migration-plan.md W9): the English source
 *       texts in the code (markup text, text attributes, literals marked `@wc-include`) are
 *       replaced by their German translation from Moliri's `src/locales/de.po`; the `@wc-…`
 *       comments go. Untranslated texts stay English. `$lib/i18n/locale.js` becomes
 *       `$lib/moliri/locale.js`, whose `getLocale()` always says 'de'.
 *     - Paraglide (Moliri before the switch): `…/paraglide/messages` becomes `$lib/moliri/texte.js`,
 *       the German texts the mirrored files call, generated from Moliri's `messages/de.json`;
 *       `…/paraglide/runtime` becomes `$lib/moliri/locale.js` as above.
 * Line endings become LF.
 *
 * WHERE MOLIRI LIVES. `MOLIRI_SRC` (path to a checkout of moliri_sveltekit), otherwise
 * `../MOLIRI-DB/moliri_sveltekit` next to the folder this repository sits in (also from a git
 * worktree). Mirroring and checking need a Moliri checkout; the build needs neither, the mirrors are
 * checked in.
 *
 *   node scripts/sync-moliri.mjs          mirror (pnpm sync:moliri)
 *   node scripts/sync-moliri.mjs --check  check only (pnpm sync:check, exit 1 on a difference)
 */

import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { parse } from 'svelte/compiler';

const frontend = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const TARGET = path.join(frontend, 'src', 'lib', 'moliri');

/** Mirrored files, relative to Moliri's `src/`. One more line takes in one more file. */
const FILES = [
	'lib/components/ui/Button.svelte',
	'lib/components/ui/Tabs.svelte',
	'lib/components/ui/Modal.svelte',
	// imported by Modal (paging between entries)
	'lib/components/ui/Pager.svelte',
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

const TEXTE = 'texte.js';
const LOCALE = 'locale.js';

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

/** The import paths that change (besides the texts, the header and the line endings). */
const REWRITES = [
	[/(['"])\$lib\/\.\.\/paraglide\/messages(?:\.js)?\1/g, `$1$lib/moliri/${TEXTE}$1`],
	[/(['"])(?:\$lib\/\.\.\/paraglide\/runtime|\$lib\/i18n\/locale)(?:\.js)?\1/g, `$1$lib/moliri/${LOCALE}$1`],
	[/(['"])\$lib\/components\//g, '$1$lib/moliri/components/']
];

const lf = (s) => s.replace(/\r\n/g, '\n');

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
		console.error(`Moliri-Checkout nicht gefunden (gesucht: ${candidates.join(', ')}).`);
		console.error('MOLIRI_SRC auf den Ordner von moliri_sveltekit setzen.');
		process.exit(2);
	}
	return found;
}

/** What is to be under src/lib/moliri/: path → content. */
async function expected(source) {
	const out = new Map();
	const keys = new Set();
	/** calls under a name the file also uses for something else (`const m = this.media`): texts only where they exist */
	const maybe = new Set();
	const catalogue = await deCatalogue(source);
	const untranslated = [];
	for (const file of FILES) {
		let text = lf(await readFile(path.join(source, 'src', file), 'utf8'));
		if (catalogue) {
			const german = germanTexts(text, file, catalogue);
			text = german.text;
			untranslated.push(...german.untranslated.map((t) => `${file}: ${t}`));
		}
		for (const [from, to] of REWRITES) text = text.replace(from, to);
		// the texts the file calls, under the name it imports them as (mostly m, also msg)
		for (const [, as] of text.matchAll(/import \* as (\w+) from (['"])\$lib\/moliri\/texte\.js\2/g)) {
			const shadowed = new RegExp(`\\b(?:const|let|var)\\s+${as}\\b`).test(text);
			for (const [, k] of text.matchAll(new RegExp(`\\b${as}\\.(\\w+)\\(`, 'g'))) (shadowed ? maybe : keys).add(k);
		}
		const rest = [...text.matchAll(/(['"])\$lib\/(?!moliri\/)([^'"]+)\1/g)].map((x) => x[2]);
		if (rest.length) throw new Error(`${file} importiert ${rest.join(', ')} aus Moliri; mitspiegeln oder hier umschreiben.`);
		out.set(file.replace(/^lib\//, ''), header(file) + text);
	}
	out.set(LOCALE, locale());
	if (keys.size || maybe.size) out.set(TEXTE, await texte(source, keys, maybe));
	if (untranslated.length) {
		console.warn(`Ohne deutsche Übersetzung in Moliris de.po, bleibt englisch (${untranslated.length}):`);
		for (const u of untranslated) console.warn(`  ${u}`);
	}
	return out;
}

function locale() {
	return [
		...MPL.map((l) => `// ${l}`),
		'',
		'// Generated by frontend/scripts/sync-moliri.mjs in place of Moliri\'s $lib/i18n/locale.js; do not edit.',
		'',
		'/** Language of the interface: Transcribe is German only. */',
		"export const getLocale = () => 'de';",
		''
	].join('\n');
}

/** Paraglide (Moliri before its switch to wuchale): the German texts the mirrors call as `m.<name>()`. */
async function texte(source, /** @type {Set<string>} */ sure, /** @type {Set<string>} */ maybe) {
	const de = JSON.parse(await readFile(path.join(source, 'messages', 'de.json'), 'utf8'));
	const keys = [...new Set([...sure, ...[...maybe].filter((k) => k in de)])].sort();
	const lines = [
		...MPL.map((l) => `// ${l}`),
		'',
		'// Erzeugt von frontend/scripts/sync-moliri.mjs aus Moliris messages/de.json: die Texte, die die',
		'// gespiegelten Komponenten über `m.<name>()` aufrufen. Ersetzt Paraglide; nicht von Hand ändern.',
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

// ---------------------------------------------------------------------------------------------
// wuchale: English source texts → German from src/locales/de.po

/**
 * Moliri's German catalogue, or null before its switch to wuchale (no `src/locales/de.po`, or the
 * mirrors still call Paraglide). Map: source file (relative to `src/`) → (context \x04 msgid → msgstr),
 * only translated entries.
 */
async function deCatalogue(source) {
	const file = path.join(source, 'src', 'locales', 'de.po');
	if (!existsSync(file)) return null;
	/** @type {Map<string, Map<string, string>>} */
	const byFile = new Map();
	for (const e of parsePo(lf(await readFile(file, 'utf8')))) {
		if (!e.id || !e.str || e.obsolete) continue;
		for (const ref of e.refs) {
			const rel = ref.replace(/:\d+$/, '').replace(/^src\//, '');
			if (!byFile.has(rel)) byFile.set(rel, new Map());
			byFile.get(rel).set(poKey(e.ctx, e.id), e.str);
		}
	}
	return byFile;
}

const poKey = (/** @type {string | null | undefined} */ ctx, /** @type {string} */ id) => `${ctx ?? ''}\x04${id}`;

/** Entries of a PO file: references, context, msgid, msgstr (plurals are not used by Moliri). */
function parsePo(/** @type {string} */ text) {
	const entries = [];
	for (const block of text.split(/\n\s*\n/)) {
		const e = { refs: /** @type {string[]} */ ([]), ctx: null, id: '', str: '', obsolete: false };
		/** @type {'ctx' | 'id' | 'str' | null} */
		let field = null;
		for (const line of block.split('\n')) {
			if (line.startsWith('#~')) e.obsolete = true;
			else if (line.startsWith('#:')) e.refs.push(...line.slice(2).trim().split(/\s+/).filter(Boolean));
			else if (line.startsWith('#')) continue;
			else if (/^msgid_plural\b|^msgstr\[/.test(line)) throw new Error(`Moliris de.po hat Pluralformen (${line}); sync-moliri.mjs kann sie noch nicht.`);
			else {
				const m = /^(msgctxt|msgid|msgstr)\s+(".*")$/.exec(line);
				if (m) {
					field = /** @type {const} */ ({ msgctxt: 'ctx', msgid: 'id', msgstr: 'str' })[m[1]];
					e[field] = poString(m[2]);
				} else if (field && /^".*"$/.test(line)) e[field] += poString(line);
			}
		}
		if (field) entries.push(e);
	}
	return entries;
}

const poString = (/** @type {string} */ quoted) =>
	quoted.slice(1, -1).replace(/\\(.)/g, (_, c) => ({ n: '\n', t: '\t', r: '\r' })[c] ?? c);

/**
 * The file with its interface texts in German: markup texts and text attributes as wuchale extracts
 * them (`@wuchale/svelte`, MixedVisitor), then the literals marked `@wc-include`; the `@wc-…`
 * comments removed. Throws when a translated text of the file is not found, so the mirror never
 * silently goes English.
 */
function germanTexts(/** @type {string} */ text, /** @type {string} */ file, /** @type {Map<string, Map<string, string>>} */ catalogue) {
	const cat = catalogue.get(file) ?? new Map();
	const used = new Set();
	const untranslated = [];
	const lookup = (/** @type {string | null} */ ctx, /** @type {string} */ id) => {
		const key = poKey(ctx, id);
		if (!cat.has(key)) return null;
		used.add(key);
		return cat.get(key);
	};
	if (file.endsWith('.svelte')) text = germanMarkup(text, lookup);
	text = germanLiterals(text, lookup, untranslated);
	text = text.replace(/<!--\s*@wc-[\s\S]*?-->/g, '').replace(/\/\*\s*@wc-[^*]*\*\/[ \t]*/g, '');
	const missing = [...cat.keys()].filter((k) => !used.has(k)).map((k) => JSON.stringify(k.split('\x04')[1]));
	if (missing.length) throw new Error(`${file}: Texte aus Moliris de.po nicht gefunden: ${missing.join(', ')}`);
	return { text, untranslated };
}

// --- markup

/** Elements wuchale never takes text from (its default heuristic plus Moliri's). */
const NO_TEXT = new Set(['script', 'style', 'path', 'code', 'pre', 'kbd', 'samp', 'var']);
/** Nodes wuchale leaves in place, outside any message. */
const IN_PLACE = new Set(['ConstTag', 'SnippetBlock', 'DeclarationTag']);
/** Sub-fragments of blocks and elements. */
const FRAGMENTS = ['fragment', 'consequent', 'alternate', 'body', 'fallback', 'pending', 'then', 'catch'];

const directive = (/** @type {any} */ node) => node.type === 'Comment' && node.data.trim().startsWith('@wc-');
const contextOf = (/** @type {string} */ data) => /@wc-context:\s*(.+?)\s*$/.exec(data.trim())?.[1] ?? null;
const hasLetters = (/** @type {string} */ body) =>
	(body.replace(/\{\d+\}|<\/?\d+\/?>/g, '').match(/\p{L}/gu) ?? []).length >= 2;

function germanMarkup(/** @type {string} */ src, /** @type {(ctx: string | null, id: string) => string | null} */ lookup) {
	const ast = parse(src, { modern: true });

	/** @typedef {{ start: number, end: number, text: string }} Edit */
	/** Source of [start, end) with the edits inside it applied. */
	const apply = (/** @type {number} */ start, /** @type {number} */ end, /** @type {Edit[]} */ edits) => {
		let out = '';
		let at = start;
		for (const e of [...edits].sort((a, b) => a.start - b.start)) {
			if (e.start < at || e.end > end) continue;
			out += src.slice(at, e.start) + e.text;
			at = e.end;
		}
		return out + src.slice(at, end);
	};

	/**
	 * A fragment's message as wuchale builds it: text with collapsed whitespace at the node edges,
	 * `{n}` for expressions, `<n/>` for elements and blocks, `<n>…</n>` for a child element whose own
	 * text joins the message. Null when wuchale would not take the fragment as one message.
	 */
	function message(/** @type {any[]} */ nodes, /** @type {string[]} */ names) {
		if (names.some((p) => NO_TEXT.has(p))) return null;
		const content = nodes.filter((n) => n.type !== 'Comment');
		if (nodes.some(directive)) return null;
		const count = { text: 0, expr: 0, element: 0 };
		for (const n of content) {
			if (n.type === 'Text') n.data.trim() && count.text++;
			else if (n.type === 'ExpressionTag') count.expr++;
			else if (!IN_PLACE.has(n.type)) count.element++;
		}
		if (!count.text) return null;
		let body = '';
		/** @type {any[]} */
		const exprs = [];
		/** @type {{ node: any, inner: ReturnType<typeof message> }[]} */
		const tags = [];
		for (const n of content) {
			if (n.type === 'Text') {
				const trimmed = n.data.trim();
				if ((/^\s/.test(n.data) || !trimmed) && !body.endsWith(' ')) body += ' ';
				if (trimmed) body += trimmed + (/\s$/.test(n.data) ? ' ' : '');
			} else if (n.type === 'ExpressionTag') {
				body += `{${exprs.length}}`;
				exprs.push(n);
			} else if (!IN_PLACE.has(n.type)) {
				const inner = n.fragment && /Element$|^Component$/.test(n.type) ? message(n.fragment.nodes, [...names, n.name]) : null;
				const i = tags.length;
				body += inner ? `<${i}>${inner.body}</${i}>` : `<${i}/>`;
				tags.push({ node: n, inner });
			}
		}
		body = body.trim();
		return hasLetters(body) ? { body, nodes, exprs, tags } : null;
	}

	/** The German source of a message, its expressions, elements and blocks kept (with their own texts in German). */
	function render(/** @type {string} */ german, /** @type {NonNullable<ReturnType<typeof message>>} */ msg, /** @type {string} */ quote) {
		const escape = (/** @type {string} */ s) => {
			s = s.replace(/[{}]/g, (c) => `{'${c}'}`).replace(/</g, '&lt;');
			return quote ? s.replaceAll(quote, quote === '"' ? '&quot;' : '&#39;') : s;
		};
		let out = '';
		let rest = german;
		const token = /\{(\d+)\}|<(\d+)\/>|<(\d+)>([\s\S]*?)<\/\3>/;
		for (let m = token.exec(rest); m; m = token.exec(rest)) {
			out += escape(rest.slice(0, m.index));
			if (m[1] !== undefined) {
				const n = msg.exprs[+m[1]];
				if (!n) throw new Error(`Platzhalter {${m[1]}} in "${german}" gibt es im Original nicht.`);
				out += apply(n.start, n.end, []);
			} else {
				const t = msg.tags[+(m[2] ?? m[3])];
				if (!t) throw new Error(`Element <${m[2] ?? m[3]}> in "${german}" gibt es im Original nicht.`);
				if (m[2] !== undefined || !t.inner) out += apply(t.node.start, t.node.end, nodeEdits(t.node, ancestors(t.node)));
				else {
					const inner = t.inner.nodes;
					const from = inner[0].start;
					const to = inner.at(-1).end;
					out += apply(t.node.start, from, attributeEdits(t.node)) + render(m[4], t.inner, '') + src.slice(to, t.node.end);
				}
			}
			rest = rest.slice(m.index + m[0].length);
		}
		return out + escape(rest);
	}

	/** element names from the root down to a node, for the elements wuchale skips */
	const parents = new Map();
	const ancestors = (/** @type {any} */ node) => {
		const names = [];
		for (let n = parents.get(node); n; n = parents.get(n)) if (n.name) names.unshift(n.name);
		return names;
	};

	/** @returns {Edit[]} */
	function fragmentEdits(/** @type {any[]} */ nodes, /** @type {string[]} */ names) {
		const msg = message(nodes, names);
		const content = nodes.filter((n) => n.type !== 'Comment');
		if (msg) {
			const german = lookup(null, msg.body);
			if (german !== null) {
				// the message replaces the nodes from its first to its last non-blank character
				const first = content[0];
				const last = content.at(-1);
				const start = first.type === 'Text' ? first.start + (first.raw.length - first.raw.trimStart().length) : first.start;
				const end = last.type === 'Text' ? last.end - (last.raw.length - last.raw.trimEnd().length) : last.end;
				return [{ start, end, text: render(german, msg, '') }];
			}
		}
		// single texts (with a @wc-context comment before them), and whatever the children hold
		const edits = [];
		let ctx = null;
		for (const n of nodes) {
			if (directive(n)) ctx = contextOf(n.data) ?? ctx;
			else if (n.type === 'Text') {
				const trimmed = n.data.trim();
				const german = trimmed && !names.some((p) => NO_TEXT.has(p)) ? lookup(ctx, trimmed) : null;
				if (german !== null) {
					const lead = n.raw.length - n.raw.trimStart().length;
					const trail = n.raw.length - n.raw.trimEnd().length;
					edits.push({ start: n.start + lead, end: n.end - trail, text: render(german, { body: trimmed, nodes: [n], exprs: [], tags: [] }, '') });
				}
				if (trimmed) ctx = null;
			} else {
				edits.push(...nodeEdits(n, names));
				ctx = null;
			}
		}
		return edits;
	}

	/** @returns {Edit[]} */
	function attributeEdits(/** @type {any} */ node) {
		const edits = [];
		for (const a of node.attributes ?? []) {
			if (a.type !== 'Attribute' || !Array.isArray(a.value) || !a.value.length) continue;
			const texts = a.value.filter((v) => v.type === 'Text');
			if (!texts.some((t) => t.data.trim())) continue;
			const exprs = a.value.filter((v) => v.type === 'ExpressionTag');
			let i = 0;
			const body = a.value.map((v) => (v.type === 'Text' ? v.data : `{${i++}}`)).join('').trim();
			const german = lookup(null, body);
			if (german === null) continue;
			const start = a.value[0].start;
			const end = a.value.at(-1).end;
			const quote = src[start - 1] === '"' || src[start - 1] === "'" ? src[start - 1] : '';
			edits.push({ start, end, text: render(german, { body, nodes: a.value, exprs, tags: [] }, quote || '"') });
		}
		return edits;
	}

	/** @returns {Edit[]} */
	function nodeEdits(/** @type {any} */ node, /** @type {string[]} */ names) {
		if (IN_PLACE.has(node.type) && node.type !== 'SnippetBlock') return [];
		const inside = node.name && /Element$|^Component$/.test(node.type) ? [...names, node.name] : names;
		const edits = attributeEdits(node);
		for (const key of FRAGMENTS) {
			const f = node[key];
			if (f?.type === 'Fragment') {
				for (const child of f.nodes) parents.set(child, node);
				edits.push(...fragmentEdits(f.nodes, inside));
			}
		}
		return edits;
	}

	const walk = (/** @type {any} */ node) => {
		for (const key of FRAGMENTS) for (const child of node[key]?.nodes ?? []) {
			parents.set(child, node);
			walk(child);
		}
	};
	walk(ast);
	return apply(0, src.length, fragmentEdits(ast.fragment.nodes, []));
}

// --- script: literals marked @wc-include

/** Where the JS literal starting at `i` ends (strings, template literals with nested `${…}`). */
function literalEnd(/** @type {string} */ s, /** @type {number} */ i) {
	const q = s[i];
	let j = i + 1;
	while (j < s.length) {
		const c = s[j];
		if (c === '\\') j += 2;
		else if (c === q) return j + 1;
		else if (q === '`' && c === '$' && s[j + 1] === '{') j = expressionEnd(s, j + 2) + 1;
		else j++;
	}
	throw new Error(`Literal ohne Ende: ${s.slice(i, i + 60)}`);
}

/** Index of the `}` closing the expression that starts at `i`. */
function expressionEnd(/** @type {string} */ s, /** @type {number} */ i) {
	let depth = 0;
	for (let j = i; j < s.length; j++) {
		const c = s[j];
		if (c === "'" || c === '"' || c === '`') j = literalEnd(s, j) - 1;
		else if (c === '{') depth++;
		else if (c === '}') {
			if (!depth) return j;
			depth--;
		}
	}
	throw new Error(`Ausdruck ohne Ende: ${s.slice(i, i + 60)}`);
}

function germanLiterals(
	/** @type {string} */ src,
	/** @type {(ctx: string | null, id: string) => string | null} */ lookup,
	/** @type {string[]} */ untranslated
) {
	const mark = /(?:\/\*\s*@wc-context:\s*([^*]+?)\s*\*\/\s*)?\/\*\s*@wc-include\s*\*\/\s*(?=['"`])/g;
	let out = '';
	let at = 0;
	for (let m = mark.exec(src); m; m = mark.exec(src)) {
		const start = m.index + m[0].length;
		const end = literalEnd(src, start);
		const literal = src.slice(start, end);
		/** @type {string[]} */
		const exprs = [];
		let id;
		if (literal[0] === '`') {
			const quasis = [];
			let q = '';
			for (let j = 1; j < literal.length - 1; j++) {
				if (literal[j] === '\\') q += literal.slice(j, ++j + 1);
				else if (literal[j] === '$' && literal[j + 1] === '{') {
					const close = expressionEnd(literal, j + 2);
					quasis.push(q);
					exprs.push(germanLiterals(literal.slice(j + 2, close), lookup, untranslated));
					q = '';
					j = close;
				} else q += literal[j];
			}
			quasis.push(q);
			id = quasis.map((x, i) => cooked(x) + (i < exprs.length ? `{${i}}` : '')).join('');
		} else id = cooked(literal.slice(1, -1));
		const german = lookup(m[1] ?? null, id);
		if (german === null) {
			untranslated.push(JSON.stringify(id));
			continue;
		}
		let text;
		if (!exprs.length && !/\{\d+\}/.test(german)) {
			const quote = literal[0] === '`' ? "'" : literal[0];
			text = quote + german.replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replaceAll(quote, `\\${quote}`) + quote;
		} else {
			text =
				'`' +
				german.replace(/[`\\]|\$\{/g, (c) => `\\${c}`).replace(/\{(\d+)\}/g, (all, i) => {
					if (exprs[+i] === undefined) throw new Error(`Platzhalter ${all} in "${german}" gibt es im Original nicht.`);
					return '${' + exprs[+i] + '}';
				}) +
				'`';
		}
		out += src.slice(at, m.index) + text;
		at = end;
		mark.lastIndex = end;
	}
	return out + src.slice(at);
}

/** The value of a string literal's body (escapes resolved). */
const cooked = (/** @type {string} */ body) =>
	body.replace(/\\(u\{[0-9a-fA-F]+\}|u[0-9a-fA-F]{4}|x[0-9a-fA-F]{2}|.)/gs, (_, e) => {
		if (e[0] === 'u') return String.fromCodePoint(parseInt(e.replace(/[u{}]/g, ''), 16));
		if (e[0] === 'x' && e.length === 3) return String.fromCharCode(parseInt(e.slice(1), 16));
		return { n: '\n', t: '\t', r: '\r', b: '\b', f: '\f', v: '\v', 0: '\0', '\n': '' }[e] ?? e;
	});

// ---------------------------------------------------------------------------------------------

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
	if (want.has(rel)) continue;
	// generated files Moliri no longer needs (texte.js after the switch to wuchale) go
	if (!check && rel === TEXTE) {
		await rm(path.join(TARGET, rel));
		console.log(`gelöscht: src/lib/moliri/${rel}`);
	} else problems.push(`nicht in der Liste (löschen oder in FILES aufnehmen): src/lib/moliri/${rel}`);
}

if (problems.length) {
	console.error(problems.join('\n'));
	if (check) console.error(`\nAbweichung von Moliri (${source}). pnpm sync:moliri gleicht an.`);
	process.exit(1);
}
console.log(check ? `Spiegel stimmen mit Moliri überein (${source}).` : `Gespiegelt aus ${source}.`);
