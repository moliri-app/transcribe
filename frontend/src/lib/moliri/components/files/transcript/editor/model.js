// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/model.js) von frontend/scripts/sync-moliri.mjs.
// Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri.

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Ported from transcribe-go (frontend/src/lib/editor/model.js); UI texts through paraglide.
// Das Datenmodell des Editors ohne Oberfläche: aus _final.json (Maschine) und _annotations.json
// (Menschen) entstehen die Wörter, wie sie im Text stehen. Die Regeln für Korrekturen und Zeitanker
// sind dieselben wie in server/transcribe/internal/annotate.

import * as m from '$lib/moliri/texte.js';
import { getLocale } from '$lib/moliri/texte.js';

/** @typedef {{ w: string, start: number, end: number, p?: number, speaker?: number | null, edited?: boolean, fill?: boolean, redone?: boolean, pin?: boolean, nopin?: boolean, lang?: string, lid?: string }} Wort */
/** @typedef {{ start: number, end: number, text?: string, mode?: string, drop?: number }} Edit */
/** @typedef {{ start: number, end: number, language?: string, speaker?: string | number }} Span */
/** @typedef {{ id: string, start: number, end: number, title: string, tags?: string[] }} Clip */
/** @typedef {{ speakers: Record<string, { name?: string, language?: string, merge_into?: string }>, spans: Span[], edits: Edit[], clips: Clip[] }} Annotationen */
/** @typedef {{ label: string, start: number, end: number }} Ereignis */

/** Zeit als 1:05 oder 1:02:05 */
export function fmt(/** @type {number} */ s) {
	s = Math.max(0, s || 0);
	const m = Math.floor(s / 60),
		h = Math.floor(m / 60);
	const ss = String(Math.floor(s % 60)).padStart(2, '0');
	return h ? `${h}:${String(m % 60).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}
/** Zeit aus „1:02,5“, „62.5“ oder „1:01:02“; NaN, wenn es keine ist */
export function parseT(/** @type {string} */ s) {
	const p = String(s).trim().replace(',', '.').split(':');
	return p.some((x) => x === '' || isNaN(+x)) ? NaN : p.reduce((t, x) => t * 60 + +x, 0);
}
/** Dauer: unter einer Minute in Sekunden, sonst als Zeit */
export const fmtDur = (/** @type {number} */ s) => (s < 60 ? `${Math.round(s)} s` : fmt(s));
/** Titel eines Clips ohne eigenen: die ersten sechs Wörter */
export function defaultTitle(/** @type {string} */ text) {
	const ws = text.split(/\s+/).filter(Boolean);
	return ws.length ? ws.slice(0, 6).join(' ').replace(/[.,;:!?…]+$/, '') + (ws.length > 6 ? ' …' : '') : m.te_clip_default();
}
/** Dezimalzeichen der Oberfläche */
const decimal = () => {
	try {
		return getLocale().startsWith('en') ? '.' : ',';
	} catch {
		return ',';
	}
};
/** Zeit mit Zehntel: 1:05,3 */
export const fmtT = (/** @type {number} */ x) => fmt(x) + decimal() + Math.floor((x % 1) * 10);
export const r3 = (/** @type {number} */ x) => Math.round(x * 1000) / 1000;

/** Namen der Sprachen stehen in sprachen.js (mit Mundarten und Suchnamen) */
export { langName } from './sprachen.js';

/** Name einer Voreinstellung der Erkennung */
export function presetName(/** @type {string} */ p) {
	switch (p) {
		case 'interview': return m.transcript_preset_interview();
		case 'gespraech': return m.transcript_preset_gespraech();
		case 'vortrag': return m.transcript_preset_vortrag();
		case 'musik': return m.transcript_preset_musik();
		case 'auto': return m.transcript_preset_auto();
		default: return p || m.transcript_preset_auto();
	}
}

/** Gattung einer Regiezeile in der Sprache der Oberfläche (der Dienst benennt Geräusche deutsch); Ruf und Schrei bleiben im Text unerwähnt */
export function cueName(/** @type {string} */ label) {
	switch (label) {
		case 'Musik':
		case 'Hintergrundmusik': return m.te_cue_music();
		case 'Gesang': return m.te_cue_singing();
		case 'Applaus': return m.te_cue_applause();
		case 'Lachen': return m.te_cue_laughter();
		case 'Jubel': return m.te_cue_cheering();
		default: return label;
	}
}
export const QUIET_EVENTS = ['Ruf', 'Schrei'];

const mid = (/** @type {{start: number, end: number}} */ w) => (w.start + w.end) / 2;

// Portierung von annotate.apply_edit, damit getippte Korrekturen sofort sichtbar sind
/** @param {Wort[]} ws */
export function applyEdit(ws, /** @type {number} */ a, /** @type {number} */ b, /** @type {string} */ text, mode = 'replace') {
	const inR = (/** @type {Wort} */ w) => a <= mid(w) && mid(w) <= b;
	/** @type {Wort[]} */ let old;
	/** @type {Wort[]} */ let keep;
	if (mode === 'insert') {
		old = [];
		keep = ws.map((w) => (w.start < a && a < w.end ? { ...w, end: a } : w));
	} else {
		old = ws.filter(inR);
		keep = ws.filter((w) => !inR(w));
	}
	const toks = text.split(/\s+/).filter(Boolean);
	if (!toks.length) return keep;
	const t0 = old.length ? Math.min(a, old[0].start) : a;
	let t1 = old.length ? Math.max(b, old[old.length - 1].end) : b;
	t1 = Math.max(t1, t0 + 0.05 * toks.length);
	/** @type {Record<string, number>} */ const counts = {};
	old.forEach((w) => {
		if (w.speaker != null) counts[w.speaker] = (counts[w.speaker] || 0) + 1;
	});
	/** @type {number | null | undefined} */ let spk;
	const top = Object.keys(counts).sort((x, y) => counts[y] - counts[x])[0];
	if (top === undefined) {
		/** @type {{w: Wort, d: number} | null} */ let near = null;
		for (const w of keep) {
			const d = Math.min(Math.abs(w.end - a), Math.abs(w.start - b));
			if (!near || d < near.d) near = { w, d };
		}
		spk = near ? near.w.speaker : null;
	} else spk = +top;
	const total = toks.reduce((n, t) => n + t.length, 0);
	let t = t0;
	const add = toks.map((tok) => {
		const d = ((t1 - t0) * tok.length) / total;
		const w = { w: tok, start: t, end: t + d, p: 1, edited: true, speaker: spk };
		t += d;
		return w;
	});
	return keep.concat(add).sort((x, y) => x.start - y.start);
}

// Zeitanker (dieselbe Regel wie internal/annotate/anchor.go): erstes Wort, Wort nach einer Pause oder
// einem Satzende, Wörter, auf die ein Mensch einen Anker gesetzt hat (pin); ein Wort, dem ein Mensch
// den Anker genommen hat, trägt nopin.
const ANCHOR_GAP = 0.35;
export const SENT_END = /[.!?…]["»«“”)]?$/;
/** @param {Wort[]} ws */
export function isAnchor(ws, /** @type {number} */ i) {
	const w = ws[i];
	if (w.pin) return true;
	if (w.nopin) return false;
	if (i === 0) return true;
	const p = ws[i - 1];
	return w.start - p.end >= ANCHOR_GAP || SENT_END.test(p.w);
}
/** @param {Wort[]} ws */
function nearestStart(ws, /** @type {number} */ t) {
	let best = 0.03,
		bi = -1;
	ws.forEach((w, i) => {
		const d = Math.abs(w.start - t);
		if (d <= best) {
			best = d;
			bi = i;
		}
	});
	return bi;
}
// Portierung von annotate.MoveAnchor: das Wort, das bei from beginnt, beginnt nun bei to; die Wörter bis
// zu den benachbarten Ankern dehnen sich linear. drop >= 0: im Text versetzt, das Wort bei drop gibt
// seinen Anker ab.
/** @param {Wort[]} ws */
export function moveAnchor(ws, /** @type {number} */ from, /** @type {number} */ to, drop = -1) {
	const out = ws.map((w) => ({ ...w })).sort((x, y) => x.start - y.start);
	const i = nearestStart(out, from);
	if (i < 0) return out;
	let j = drop >= 0 ? nearestStart(out, drop) : -1;
	if (j === i) j = -1;
	if (j >= 0) {
		delete out[j].pin;
		out[j].nopin = true;
	}
	let prev = -1,
		next = out.length;
	for (let k = i - 1; k >= 0; k--)
		if (k !== j && isAnchor(out, k)) {
			prev = k;
			break;
		}
	for (let k = i + 1; k < out.length; k++)
		if (k !== j && isAnchor(out, k)) {
			next = k;
			break;
		}
	let sPrev = 0,
		tPrev = 0;
	if (prev >= 0) sPrev = tPrev = out[prev].start;
	else if (i > 0) sPrev = tPrev = out[0].start;
	const sNext = next < out.length ? out[next].start : out[out.length - 1].end,
		tNext = sNext,
		sI = out[i].start;
	to = Math.max(tPrev + 0.02 * (i - Math.max(prev, 0)), Math.min(to, tNext - 0.02 * (next - i)));
	const f = (/** @type {number} */ x) =>
		x <= sPrev
			? x
			: x < sI
				? sI - sPrev < 1e-6
					? tPrev
					: tPrev + ((x - sPrev) * (to - tPrev)) / (sI - sPrev)
				: x < sNext
					? sNext - sI < 1e-6
						? to
						: to + ((x - sI) * (tNext - to)) / (sNext - sI)
					: x;
	for (let k = Math.max(prev, 0); k < next; k++) {
		const ns = f(out[k].start);
		out[k].end = r3(Math.max(ns + 0.01, f(Math.min(out[k].end, sNext))));
		out[k].start = r3(ns);
	}
	out[i].start = r3(to);
	delete out[i].nopin;
	out[i].pin = true;
	return out.sort((x, y) => x.start - y.start);
}

const editKey = (/** @type {Edit} */ e) =>
	[(+e.start).toFixed(3), (+e.end).toFixed(3), e.text || '', e.mode || 'replace', e.drop ?? ''].join('|');

/**
 * Die Wörter, wie sie im Text stehen: Maschine plus menschliche Korrekturen.
 * _final.json enthält schon die Korrekturen des letzten „Neu berechnen“: nur die späteren kommen
 * obendrauf (eine Einfügung darf nicht doppelt erscheinen). Wurde eine angewandte Korrektur
 * seitdem entfernt, gelten wieder alle.
 * @returns {{ words: Wort[], anchors: number[], appliedN: number }}
 */
export function buildWords(/** @type {any} */ final, /** @type {Annotationen} */ ann) {
	/** @type {Wort[]} */
	let ws = final.segments.flatMap((/** @type {any} */ s) => s.words.map((/** @type {Wort} */ w) => ({ ...w }))).sort((/** @type {Wort} */ x, /** @type {Wort} */ y) => x.start - y.start);
	const done = final.edits_applied || [];
	let k = 0;
	while (k < done.length && k < ann.edits.length && editKey(done[k]) === editKey(ann.edits[k])) k++;
	const appliedN = k === done.length ? k : 0;
	for (const e of ann.edits.slice(appliedN))
		ws = e.mode === 'anchor' ? moveAnchor(ws, e.start, e.end, e.drop ?? -1) : applyEdit(ws, e.start, e.end, e.text || '', e.mode);
	for (const s of ann.spans)
		if (s.speaker !== undefined && s.speaker !== '')
			ws.forEach((w) => {
				const m = mid(w);
				if (s.start <= m && m <= s.end) w.speaker = +s.speaker;
			});
	const anchors = ws.map((_, i) => (isAnchor(ws, i) ? i : -1)).filter((i) => i >= 0);
	return { words: ws, anchors, appliedN };
}

/**
 * Sprache je Wort: von Hand gesetzt > neu erkannt > von der Maschine erkannt > Deutsch
 * @returns {{ lang: string, by: 'mensch' | 'maschine' | 'standard' }}
 */
export function wordLang(/** @type {Wort} */ w, /** @type {Annotationen} */ ann) {
	const m = mid(w);
	const span = ann.spans.find((s) => s.language && s.start <= m && m <= s.end);
	if (span?.language) return { lang: span.language, by: 'mensch' };
	const sl = w.speaker != null ? ann.speakers[w.speaker]?.language : null;
	if (sl) return { lang: sl, by: 'mensch' };
	if (w.lang) return { lang: w.lang, by: 'mensch' };
	if (w.lid && w.lid !== 'de' && w.lid !== '?') return { lang: w.lid, by: 'maschine' };
	return { lang: 'de', by: 'standard' };
}

/** @param {Wort[]} words */
export function speakerIds(words) {
	return [...new Set(words.map((w) => w.speaker).filter((s) => s != null))].sort((a, b) => /** @type {number} */ (a) - /** @type {number} */ (b));
}

/** Sprechzeit je Sprecher in Sekunden */
export function speakingTime(/** @type {Wort[]} */ words) {
	/** @type {Map<number, number>} */ const tot = new Map();
	for (const w of words) if (w.speaker != null) tot.set(w.speaker, (tot.get(w.speaker) || 0) + w.end - w.start);
	return tot;
}

/**
 * Einträge des Texts in zeitlicher Folge: Wörter und Ereignisse. Ein Ereignis allein wird eine
 * Regiezeile (cue), unter Sprache bleibt es eine kleine Notiz im Fließtext.
 * @typedef {Wort & { kind: 'w' | 'ev', wi?: number, cue?: boolean, label?: string }} Eintrag
 * @returns {Eintrag[]}
 */
export function buildItems(/** @type {Wort[]} */ words, /** @type {Ereignis[]} */ events) {
	/** @type {Eintrag[]} */
	const items = words.map((w, wi) => ({ ...w, kind: 'w', wi }));
	for (const e of events || []) {
		if (QUIET_EVENTS.includes(e.label)) continue;
		let spoken = 0;
		for (const w of words) if (w.start < e.end && w.end > e.start) spoken += Math.min(w.end, e.end) - Math.max(w.start, e.start);
		const under = spoken > 0.5 * (e.end - e.start);
		items.push({
			kind: 'ev',
			label: e.label,
			w: under ? `(${e.label === 'Musik' ? m.te_background_music() : cueName(e.label)})` : e.label,
			start: e.start,
			end: e.end,
			cue: !under
		});
	}
	return items.sort((a, b) => a.start - b.start);
}

/** letzter Eintrag, der bis t begonnen hat (binäre Suche); -1 vor dem ersten */
export function itemAt(/** @type {{start: number}[]} */ items, /** @type {number} */ t) {
	let lo = 0,
		hi = items.length - 1,
		ans = -1;
	while (lo <= hi) {
		const m = (lo + hi) >> 1;
		if (items[m].start <= t) {
			ans = m;
			lo = m + 1;
		} else hi = m - 1;
	}
	return ans;
}

/**
 * Der Text als Folge von Blöcken: Absätze (neu bei Sprecherwechsel, nach einer Regiezeile, nach einem
 * Satzende mit langer Pause oder nach 80 Wörtern), Regiezeilen und eingeklappte Stellen ausgeblendeter
 * Sprecher. hidden: Sprecher, die ausgeblendet sind; isOpened: ob eine eingeklappte Stelle bei t
 * wieder aufgeklappt wurde.
 * @typedef {{ kind: 'absatz', key: string, spk: number | null | undefined, weiter: boolean, aus: boolean, start: number, idx: number[] }} Absatz
 * @typedef {{ kind: 'cue', key: string, aus: boolean, start: number, end: number, i: number, label: string }} Regie
 * @typedef {{ kind: 'klappe', key: string, start: number, end: number, spk: Set<number>, n: number, open: boolean }} Klappe
 * @typedef {Absatz | Regie | Klappe} Block
 * @returns {{ blocks: Block[], groups: Klappe[], folded: Set<number> }}
 */
export function buildBlocks(/** @type {Eintrag[]} */ items, /** @type {Set<number>} */ hidden, /** @type {(t: number) => boolean} */ isOpened) {
	/** @type {Block[]} */ const blocks = [];
	/** @type {Klappe[]} */ const groups = [];
	/** @type {Set<number>} */ const folded = new Set(); // Einträge in einer zugeklappten Stelle
	const filtering = hidden.size > 0;
	/** @type {Absatz | null} */ let block = null;
	/** @type {Klappe | null} */ let grp = null;
	/** @type {number | null | undefined} */ let lastSpk;
	/** @type {Eintrag | null} */ let prevW = null;
	let n = 0;
	const newBlock = (/** @type {Eintrag} */ it, /** @type {number} */ i, /** @type {number | null | undefined} */ spk, /** @type {boolean} */ weiter) => {
		block = { kind: 'absatz', key: 'a' + i, spk, weiter, aus: !!grp, start: it.start, idx: [] };
		blocks.push(block);
		n = 0;
	};
	items.forEach((it, i) => {
		const hide = filtering && (it.kind === 'w' ? it.speaker != null && hidden.has(it.speaker) : !!grp);
		if (hide && !grp) {
			grp = { kind: 'klappe', key: 'k' + i, start: it.start, end: it.end, spk: new Set(), n: 0, open: isOpened(it.start) };
			blocks.push(grp);
			groups.push(grp);
			block = null;
			lastSpk = undefined;
		} else if (!hide && grp) {
			grp = null;
			block = null;
			lastSpk = undefined;
		}
		if (grp) {
			grp.end = Math.max(grp.end, it.end);
			if (it.kind === 'w') {
				if (it.speaker != null) grp.spk.add(it.speaker);
				grp.n++;
			}
			if (!grp.open) {
				folded.add(i);
				return;
			}
		}
		if (it.kind === 'ev' && it.cue) {
			blocks.push({ kind: 'cue', key: 'c' + i, aus: !!grp, start: it.start, end: it.end, i, label: it.label || it.w });
			block = null; // Sprache nach einer Regiezeile beginnt einen neuen Absatz
			return;
		}
		const spk = it.kind === 'w' ? it.speaker : lastSpk;
		const sentenceEnd = prevW && SENT_END.test(prevW.w);
		const brk = it.kind === 'w' && sentenceEnd && prevW && (it.start - prevW.end > 2 || n >= 80);
		if (!block || (it.kind === 'w' && spk !== lastSpk)) newBlock(it, i, spk, false);
		else if (brk) newBlock(it, i, spk, true);
		if (it.kind === 'w') {
			lastSpk = spk;
			prevW = it;
			n++;
		}
		/** @type {Absatz} */ (/** @type {unknown} */ (block)).idx.push(i);
	});
	return { blocks, groups, folded };
}

/**
 * Wo das Abspielen laufen darf, solange Sprecher ausgeblendet sind: Stellen sichtbarer Sprecher und
 * wieder aufgeklappte Stellen. null ohne Filter.
 * @returns {{a: number, b: number}[] | null}
 */
export function playRanges(/** @type {Wort[]} */ words, /** @type {Set<number>} */ hidden, /** @type {Klappe[]} */ groups) {
	if (!hidden.size) return null;
	/** @type {{a: number, b: number}[]} */ const out = [];
	/** @type {{a: number, b: number} | null} */ let r = null;
	for (const w of words) {
		if (w.speaker != null && hidden.has(w.speaker)) {
			r = null;
			continue;
		}
		if (r && w.start - r.b < 1.5) r.b = Math.max(r.b, w.end + 0.25);
		else out.push((r = { a: Math.max(0, w.start - 0.1), b: w.end + 0.25 }));
	}
	for (const g of groups) if (g.open) out.push({ a: g.start, b: g.end });
	out.sort((x, y) => x.a - y.a);
	return out.reduce((/** @type {{a: number, b: number}[]} */ m, x) => {
		const l = m[m.length - 1];
		if (l && x.a <= l.b) l.b = Math.max(l.b, x.b);
		else m.push({ ...x });
		return m;
	}, []);
}

/**
 * Einfügestelle zwischen zwei Wörtern. Parakeet lässt ein Wort dort enden, wo das nächste beginnt;
 * das wahre Ende des vorigen wird geschätzt (~70 ms pro Buchstabe), der Rest gilt als Pause.
 */
export function gapRange(/** @type {Eintrag[]} */ items, /** @type {number} */ i) {
	const prev = items[i],
		next = items.slice(i + 1).find((it) => it.kind === 'w');
	const b = next ? next.start : prev.end + 1;
	const est = prev.kind === 'w' ? prev.start + 0.15 + 0.07 * prev.w.length : prev.end;
	return { a: Math.max(prev.start + 0.1, Math.min(est, prev.end, b - 0.3)), b };
}

/** Läufe gleicher Eigenschaft (Sprecher, Sprache) für die Zeitleiste */
export function runs(/** @type {Wort[]} */ words, /** @type {(w: Wort) => any} */ key, gap = 1.5) {
	/** @type {any[]} */ const out = [];
	for (const w of words) {
		const k = key(w);
		if (k == null) continue;
		const r = out[out.length - 1];
		if (r && r.k === k.k && w.start - r.end < gap) r.end = w.end;
		else out.push({ ...k, start: w.start, end: w.end });
	}
	return out;
}

/**
 * Korrekturen aus einem direkt bearbeiteten Absatz: die getippten Wörter werden wortweise mit dem
 * verglichen, was der Absatz zeigte (längste gemeinsame Teilfolge). Jede geänderte Strecke wird eine
 * Korrektur: ersetzt über die Zeit der alten Wörter, oder eingefügt in die Pause nach dem Wort davor.
 * Eine ältere Korrektur in denselben Wörtern wird abgelöst (außer sie steckt schon in _final.json).
 * @param {Eintrag[]} items alle Einträge des Texts
 * @param {number[]} idx die Wort-Einträge des Absatzes
 * @param {string[]} old was der Absatz je Wort zeigte
 * @param {string[]} neu die Wörter nach dem Tippen
 * @param {Edit[]} edits bisherige Korrekturen
 * @param {number} appliedN wie viele davon schon in _final.json stecken
 * @returns {Edit[] | null} die neuen Korrekturen, null wenn sich nichts geändert hat
 */
export function textEdits(items, idx, old, neu, edits, appliedN) {
	const n = old.length,
		m = neu.length;
	const L = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
	for (let a = n - 1; a >= 0; a--) for (let b = m - 1; b >= 0; b--) L[a][b] = old[a] === neu[b] ? L[a + 1][b + 1] + 1 : Math.max(L[a + 1][b], L[a][b + 1]);
	/** @type {{ at: number, o: number[], n: string[] }[]} */ const hunks = [];
	/** @type {{ at: number, o: number[], n: string[] } | null} */ let h = null;
	let a = 0,
		b = 0;
	while (a < n || b < m) {
		if (a < n && b < m && old[a] === neu[b]) {
			if (h) hunks.push(h);
			h = null;
			a++;
			b++;
			continue;
		}
		h ??= { at: a, o: [], n: [] };
		if (b < m && (a >= n || L[a][b + 1] >= L[a + 1][b])) h.n.push(neu[b++]);
		else h.o.push(idx[a++]);
	}
	if (h) hunks.push(h);
	if (!hunks.length) return null;
	let out = edits.slice();
	const prevWord = (/** @type {number} */ i) => {
		for (let k = i - 1; k >= 0; k--) if (items[k].kind === 'w') return k;
		return null;
	};
	// Getippt direkt neben Wörtern, die schon von Hand stehen (etwa nach einer Übernahme in der
	// Tipppause weitergeschrieben): deren Korrektur wächst um den neuen Text. Eine zweite Einfügung
	// fände in der Pause dahinter oft keinen Platz mehr und rutschte hinter das nächste Wort.
	for (const hk of hunks) {
		if (hk.o.length) continue;
		let l = hk.at,
			r = hk.at;
		while (l > 0 && items[idx[l - 1]].edited) l--;
		while (r < n && items[idx[r]].edited) r++;
		if (l === r) continue;
		hk.n = [...old.slice(l, hk.at), ...hk.n, ...old.slice(hk.at, r)];
		hk.o = idx.slice(l, r);
	}
	for (const hk of hunks) {
		const fillish = hk.o.some((i) => items[i].fill);
		const text = hk.n.map((t) => (fillish ? t.replace(/^\(|\)$/g, '') : t)).join(' ');
		if (hk.o.length) {
			const s0 = items[hk.o[0]].start,
				s1 = items[hk.o[hk.o.length - 1]].end;
			const inside = (/** @type {Edit} */ e, /** @type {number} */ k) => k >= appliedN && e.mode !== 'anchor' && e.start >= s0 - 0.01 && e.end <= s1 + 0.01;
			const covered = out.filter(inside);
			out = out.filter((e, k) => !inside(e, k));
			// waren es nur Einfügungen, bleibt es eine, über deren Pausen
			if (covered.length && covered.every((e) => e.mode === 'insert')) {
				const last = hk.o[hk.o.length - 1],
					next = items.slice(last + 1).find((it) => it.kind === 'w'),
					before = prevWord(hk.o[0]);
				out.push(fitInsert(Math.min(...covered.map((e) => e.start)), next ? next.start : Math.max(...covered.map((e) => e.end)), hk.n.length, before != null ? items[before].start : 0, text));
			} else out.push({ start: r3(s0), end: r3(s1), text });
		} else {
			const before = hk.at > 0 ? idx[hk.at - 1] : prevWord(idx[0]);
			const r = before != null ? gapRange(items, before) : { a: Math.max(0, items[idx[0]].start - 0.6), b: items[idx[0]].start };
			out.push(fitInsert(r.a, r.b, hk.n.length, before != null ? items[before].start : 0, text));
		}
	}
	return out;
}

/**
 * Eine Einfügung in die Pause [a, b] vor dem nächsten Wort (das bei b beginnt). Die neuen Wörter
 * teilen sich die Zeit nach ihrer Länge, jedes mindestens 50 ms (wie annotate.ApplyEdit); ist die
 * Pause dafür zu kurz, beginnt die Einfügung früher, im Wort davor (lo: dessen Beginn). So beginnen
 * alle neuen Wörter vor dem nächsten und stehen im Text an der getippten Stelle.
 * @returns {Edit}
 */
function fitInsert(/** @type {number} */ a, /** @type {number} */ b, /** @type {number} */ k, /** @type {number} */ lo, /** @type {string} */ text) {
	const start = Math.max(lo + 0.02, Math.min(a, b - 0.05 * k - 0.01));
	return { start: r3(start), end: r3(Math.max(b, start + 0.05 * k)), text, mode: 'insert' };
}

/** Wörter, deren Mitte im Bereich liegt, als Zitat */
export function rangeText(/** @type {Wort[]} */ words, /** @type {number} */ a, /** @type {number} */ b) {
	return words
		.filter((w) => a <= mid(w) && mid(w) <= b)
		.map((w) => w.w)
		.join(' ');
}

/**
 * Wie weit ein Zeitanker wandern darf: bis kurz vor die benachbarten Anker (20 ms je Wort dazwischen,
 * wie annotate.MoveAnchor). prevW/nextW: Wortnummern der Nachbaranker.
 */
export function anchorBounds(/** @type {Wort[]} */ words, /** @type {number[]} */ anchors, /** @type {number} */ i) {
	const k = anchors.indexOf(i),
		prev = k > 0 ? anchors[k - 1] : -1,
		next = k + 1 < anchors.length ? anchors[k + 1] : words.length;
	const lo = prev >= 0 ? words[prev].start + 0.02 * (i - prev) : 0;
	const hi = (next < words.length ? words[next].start : words[words.length - 1].end) - 0.02 * (next - i);
	return { lo, hi, prevW: prev, nextW: next };
}

/** Beschreibung einer Korrektur für die Liste; was ein Mensch geschrieben hat, steht in em */
export function describeEdit(/** @type {Edit} */ e) {
	if (e.mode === 'anchor')
		return e.drop != null
			? { t: e.end, what: m.te_corr_anchor_moved(), em: '', after: fmtT(e.end) }
			: { t: e.end, what: m.te_corr_anchor({ from: fmtT(e.start) }), em: fmtT(e.end), after: '' };
	if (!e.text) return { t: e.start, what: '', em: m.te_corr_deleted(), after: '' };
	return { t: e.start, what: e.mode === 'insert' ? m.te_corr_inserted() : m.te_corr_replaced(), em: e.text, after: '' };
}

// ---------------- Suche ----------------

/** Lower case without accents, so „Muller“ finds „Müller“. */
export function foldText(/** @type {string} */ text) {
	return String(text ?? '')
		.normalize('NFD')
		.replace(/\p{M}/gu, '')
		.toLocaleLowerCase('de');
}

/**
 * Search hits in the text: each hit is the range of items [from, to] it covers. A query of
 * several words finds them in a row, across punctuation; markers are not searched.
 * @param {Eintrag[]} items
 * @param {string} query
 * @returns {{ from: number, to: number }[]}
 */
export function searchItems(items, query) {
	const needle = foldText(query).replace(/\s+/g, ' ').trim();
	if (!needle) return [];
	/** @type {number[]} */
	const owner = []; // item index per character of the haystack
	let haystack = '';
	items.forEach((item, i) => {
		if (item.kind !== 'w') return;
		if (haystack) {
			haystack += ' ';
			owner.push(i);
		}
		const text = foldText(item.w);
		haystack += text;
		for (let k = 0; k < text.length; k++) owner.push(i);
	});
	/** @type {{ from: number, to: number }[]} */
	const hits = [];
	let at = haystack.indexOf(needle);
	while (at >= 0) {
		hits.push({ from: owner[at], to: owner[at + needle.length - 1] });
		at = haystack.indexOf(needle, at + needle.length);
	}
	return hits;
}
