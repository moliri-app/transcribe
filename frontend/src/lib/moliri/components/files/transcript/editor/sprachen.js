// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/sprachen.js) von frontend/scripts/sync-moliri.mjs.
// Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri.

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Ported from transcribe-go (frontend/src/lib/editor/sprachen.js); the names follow the UI language.
// Languages for speakers and passages: code, name and other names one searches by („Platt“ finds
// Plattdeutsch). The machine recognises again only what Whisper knows; the same list is in the
// transcribe service (server/transcribe/internal/annotate/languages.go). Everything else, dialects
// above all, is noted and otherwise stays as the machine heard it.

import { getLocale } from '$lib/moliri/texte.js';

/** @typedef {{ code: string, name: string, also: string[], neu: boolean }} Sprache */

const WHISPER = new Set(
	`en zh de es ru ko fr ja pt tr pl ca nl ar sv it id hi fi vi he uk el ms cs ro da hu ta no th ur hr bg
	lt la mi ml cy sk te fa lv bn sr az sl kn et mk br eu is hy ne mn bs kk sq sw gl mr pa si km sn yo so af
	oc ka be tg sd gu am yi lo uz fo ht ps tk nn mt sa lb my bo tl mg as tt haw ln ha ba jw su yue`.split(/\s+/)
);

/** German names before the browser's: dialects and languages called otherwise here (also search aliases in every UI language) */
const EIGEN = /** @type {Record<string, string[]>} */ ({
	de: ['Deutsch', 'Hochdeutsch'],
	nds: ['Plattdeutsch', 'Niederdeutsch', 'Platt'],
	gsw: ['Alemannisch', 'Schweizerdeutsch', 'Schwyzerdütsch', 'Elsässisch', 'Badisch'],
	bar: ['Bairisch', 'Bayerisch', 'Österreichisch'],
	swg: ['Schwäbisch'],
	ksh: ['Kölsch'],
	pfl: ['Pfälzisch'],
	sli: ['Schlesisch'],
	frr: ['Nordfriesisch', 'Friesisch'],
	stq: ['Saterfriesisch', 'Friesisch'],
	fy: ['Westfriesisch', 'Friesisch'],
	hsb: ['Obersorbisch', 'Sorbisch', 'Wendisch'],
	dsb: ['Niedersorbisch', 'Sorbisch', 'Wendisch'],
	rom: ['Romani', 'Romanes'],
	yi: ['Jiddisch'],
	lb: ['Luxemburgisch'],
	en: ['Englisch'],
	zh: ['Chinesisch', 'Mandarin'],
	yue: ['Kantonesisch'],
	fa: ['Persisch', 'Farsi'],
	he: ['Hebräisch', 'Iwrit'],
	no: ['Norwegisch'],
	nn: ['Norwegisch (Nynorsk)', 'Nynorsk'],
	jw: ['Javanisch'],
	tl: ['Tagalog', 'Filipino'],
	ku: ['Kurdisch'],
	ti: ['Tigrinya'],
	prs: ['Dari']
});

/** offered first, before anyone types */
const HAEUFIG = ['de', 'nds', 'en', 'fr', 'nl', 'it', 'es', 'pl', 'tr', 'ru', 'uk', 'ar'];

/** @type {Map<string, Map<string, Sprache>>} */
const KATALOGE = new Map();
/** @type {Map<string, Intl.DisplayNames | null>} */
const BROWSER = new Map();

function locale() {
	try {
		return getLocale() || 'de';
	} catch {
		return 'de';
	}
}

function browserName(/** @type {string} */ c) {
	const loc = locale();
	if (!BROWSER.has(loc)) {
		try {
			BROWSER.set(loc, new Intl.DisplayNames([loc], { type: 'language', fallback: 'none' }));
		} catch {
			// without Intl.DisplayNames the own names and the codes remain
			BROWSER.set(loc, null);
		}
	}
	try {
		const name = BROWSER.get(loc)?.of(c) || '';
		return name ? name.charAt(0).toLocaleUpperCase(loc) + name.slice(1) : '';
	} catch {
		return '';
	}
}

/** The catalogue in the UI language: in German the own names lead, elsewhere the browser's. */
function katalog() {
	const loc = locale();
	let k = KATALOGE.get(loc);
	if (k) return k;
	k = new Map();
	const german = loc.startsWith('de');
	for (const code of new Set([...Object.keys(EIGEN), ...WHISPER])) {
		const own = EIGEN[code] ?? [];
		const b = browserName(code);
		const name = (german ? own[0] : b) || b || own[0];
		if (!name) continue;
		const also = [...own, ...(b ? [b] : [])].filter((n, i, all) => n !== name && all.indexOf(n) === i);
		k.set(code, { code, name, also, neu: WHISPER.has(code) });
	}
	KATALOGE.set(loc, k);
	return k;
}

/** can the machine recognise a passage in this language again? */
export const recognizable = (/** @type {string} */ c) => WHISPER.has(c);

/** name of a language, else its code */
export const langName = (/** @type {string} */ c) => katalog().get(c)?.name || browserName(c) || c;

/** a language for a code, also for codes outside the list */
export function sprache(/** @type {string} */ c) {
	return katalog().get(c) ?? { code: c, name: browserName(c) || c, also: [], neu: WHISPER.has(c) };
}

const norm = (/** @type {string} */ s) =>
	s
		.toLowerCase()
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '');

/**
 * Suggestions for an input: code, start of the name, start of another name, then anywhere in it.
 * Two or three letters that are no known language are offered as a code of their own.
 * @returns {(Sprache & { eigen?: boolean })[]}
 */
export function vorschlaege(/** @type {string} */ eingabe, max = 8) {
	const q = norm(eingabe.trim().replace(/\s*\(([a-z]{2,3})\)\s*$/i, ' $1'));
	if (!q) return HAEUFIG.map((c) => sprache(c));
	/** @type {[number, Sprache][]} */
	const hits = [];
	for (const s of katalog().values()) {
		const n = norm(s.name),
			also = s.also.map(norm);
		const rank =
			s.code === q ? 0 : n.startsWith(q) ? 1 : also.some((a) => a.startsWith(q)) ? 2 : n.includes(q) ? 3 : also.some((a) => a.includes(q)) ? 4 : -1;
		if (rank >= 0) hits.push([rank, s]);
	}
	hits.sort((a, b) => a[0] - b[0] || +HAEUFIG.includes(b[1].code) - +HAEUFIG.includes(a[1].code) || a[1].name.localeCompare(b[1].name, locale()));
	/** @type {(Sprache & { eigen?: boolean })[]} */
	const out = hits.slice(0, max).map((h) => h[1]);
	if (/^[a-z]{2,3}$/.test(q) && !katalog().has(q)) out.push({ ...sprache(q), eigen: true });
	return out;
}
