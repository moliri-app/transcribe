// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Finden in den Live-Aufnahmen: nach Herkunft filtern, sortieren und unscharf im ganzen Text suchen
// (fuzzysort). Gesucht wird Wort für Wort, nicht in ganzen Sätzen: In einem langen Satz fände sich
// fast jede Buchstabenfolge irgendwo verstreut, und echte Tippfehler („Absperung“) wären davon nicht
// zu unterscheiden. Mehrere Suchwörter müssen alle in derselben Aufnahme vorkommen.
import fuzzysort from 'fuzzysort';
import { art, laenge, sprachname, vonHand, wer } from './stand.svelte.js';

/** @typedef {import('./stand.svelte.js').Notiz} Notiz */
/** zeile: der Satz, der die Suchwörter am besten trifft (-1: Treffer nur im Namen)
 * @typedef {{ score: number, zeile: number }} Treffer */

export const HERKUNFT = /** @type {const} */ ({ alle: 'Alle', hand: 'Von Hand', api: 'Per API' });
/** @typedef {keyof typeof HERKUNFT} Herkunft */

export const SORTIERUNG = /** @type {const} */ ({
	neu: 'Neueste zuerst',
	alt: 'Älteste zuerst',
	lang: 'Längste zuerst'
});
/** @typedef {keyof typeof SORTIERUNG} Sortierung */

/** Treffer darunter sind zu verstreut, um gemeint zu sein; ein fehlender oder doppelter Buchstabe liegt darüber */
const SCHWELLE = 0.3;

/** Wörter; Bindestriche trennen, damit „Holländer-Mahlwerk“ nicht als ein langes Wort alles trifft */
const WORT = /[\p{L}\p{N}]+/gu;

/** @param {string} q */
const begriffe = (q) => q.toLowerCase().match(WORT) ?? [];

/** Ein oder zwei Buchstaben finden sich in fast jedem Wort: sie müssen dicht beieinander stehen.
 * @param {string} begriff @param {number} score */
const reicht = (begriff, score) => score >= (begriff.length < 3 ? 0.8 : SCHWELLE);

/** @param {Notiz} n @param {Herkunft} h */
export function passt(n, h) {
	return h === 'alle' || (h === 'hand') === vonHand(n);
}

/** @param {Notiz[]} liste @param {Sortierung} s @param {number} jetzt */
export function sortieren(liste, s, jetzt) {
	const zeit = (/** @type {Notiz} */ n) => new Date(n.started).getTime();
	if (s === 'alt') return liste.toSorted((a, b) => zeit(a) - zeit(b));
	if (s === 'lang') return liste.toSorted((a, b) => laenge(b, jetzt) - laenge(a, jetzt));
	return liste.toSorted((a, b) => zeit(b) - zeit(a));
}

/** Sucht in allen Sätzen und in Name, Art und Sprache jeder Aufnahme.
 * @param {Notiz[]} notizen @param {string} q
 * @returns {Map<string, Treffer>} je Aufnahme der beste Treffer */
export function suchen(notizen, q) {
	/** @type {Map<string, Treffer>} */
	const beste = new Map();
	const b = begriffe(q);
	if (!b.length) return beste;

	/** jedes Wort und wo es steht
	 * @type {Map<string, { id: string, zeile: number }[]>} */
	const wo = new Map();
	/** @param {string} text @param {string} id @param {number} zeile */
	const merken = (text, id, zeile) => {
		for (const [w] of text.matchAll(WORT)) {
			const l = wo.get(w);
			if (l) l.push({ id, zeile });
			else wo.set(w, [{ id, zeile }]);
		}
	};
	for (const n of notizen) {
		merken(`${wer(n)} ${n.client ?? ''} ${art(n)} ${sprachname(n.language)} ${n.audio ?? ''}`, n.id, -1);
		n.lines.forEach((l, i) => merken(l.text, n.id, i));
	}
	const woerter = [...wo.keys()];

	/** je Aufnahme: wie gut jedes Suchwort traf, insgesamt und in jedem Satz
	 * @type {Map<string, { gesamt: number[], saetze: Map<number, number[]> }>} */
	const je = new Map();
	b.forEach((begriff, k) => {
		for (const r of fuzzysort.go(begriff, woerter, { threshold: SCHWELLE })) {
			if (!reicht(begriff, r.score)) continue;
			for (const { id, zeile } of wo.get(r.target) ?? []) {
				let t = je.get(id);
				if (!t) je.set(id, (t = { gesamt: b.map(() => 0), saetze: new Map() }));
				t.gesamt[k] = Math.max(t.gesamt[k], r.score);
				if (zeile < 0) continue;
				const s = t.saetze.get(zeile) ?? b.map(() => 0);
				s[k] = Math.max(s[k], r.score);
				t.saetze.set(zeile, s);
			}
		}
	});

	const summe = (/** @type {number[]} */ l) => l.reduce((a, x) => a + x, 0);
	for (const [id, t] of je) {
		if (t.gesamt.some((x) => !x)) continue;
		// gezeigt wird der Satz, der die meisten Suchwörter am besten trifft
		let zeile = -1;
		let gut = 0;
		for (const [i, s] of t.saetze) {
			if (summe(s) > gut) [zeile, gut] = [i, summe(s)];
		}
		beste.set(id, { score: summe(t.gesamt) / b.length, zeile });
	}
	return beste;
}

/** Die Stellen der Suchwörter in einem Satz, zum Hervorheben.
 * @param {string} q @param {string} satz @returns {number[] | null} */
export function stellenIn(q, satz) {
	const b = begriffe(q);
	/** @type {number[]} */
	const stellen = [];
	for (const m of satz.matchAll(WORT)) {
		for (const begriff of b) {
			const r = fuzzysort.single(begriff, m[0]);
			if (r && reicht(begriff, r.score)) stellen.push(...r.indexes.map((i) => i + m.index));
		}
	}
	return stellen.length ? stellen : null;
}

/** Ein langer Satz, dessen Treffer weit hinten steht, beginnt kurz vor dem Treffer, damit er in
 * der Vorschau zu sehen ist.
 * @param {string} satz @param {number[]} stellen */
export function ausschnitt(satz, stellen) {
	const erste = Math.min(...stellen);
	if (!stellen.length || erste < 48) return { satz, stellen };
	const ab = satz.lastIndexOf(' ', erste - 24) + 1;
	return { satz: '… ' + satz.slice(ab), stellen: stellen.map((i) => i - ab + 2) };
}

/** Zerlegt einen Satz in Stücke, die getroffen sind und die nicht.
 * @param {string} satz @param {number[]} stellen @returns {{ t: string, an: boolean }[]} */
export function stuecke(satz, stellen) {
	const an = new Set(stellen);
	/** @type {{ t: string, an: boolean }[]} */
	const out = [];
	for (let i = 0; i < satz.length; i++) {
		const letztes = out.at(-1);
		if (letztes && letztes.an === an.has(i)) letztes.t += satz[i];
		else out.push({ t: satz[i], an: an.has(i) });
	}
	return out;
}
