// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Zugriff auf die HTTP-API der EXE (internal/server) und was mehrere Seiten daran brauchen.

/** @param {string} url @param {RequestInit} [init] */
export async function getJSON(url, init) {
	const r = await fetch(url, init);
	if (!r.ok) throw new Error(r.statusText || `Fehler ${r.status}`);
	return r.json();
}

/**
 * POST mit JSON; wirft mit der Meldung des Servers, wenn er ablehnt.
 * @param {string} url
 * @param {unknown} [body]
 */
export async function post(url, body) {
	const r = await fetch(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	const j = await r.json().catch(() => ({}));
	if (!r.ok || j.ok === false) throw new Error(j.error || r.statusText);
	return j;
}

/**
 * Lädt eine Datei nach audio/ hoch; gibt den Namen zurück, unter dem sie dort liegt.
 * @param {File} f
 * @returns {Promise<string>}
 */
export async function upload(f) {
	const r = await fetch(`/api/upload?name=${encodeURIComponent(f.name)}`, { method: 'POST', body: f });
	const j = await r.json().catch(() => ({}));
	if (!r.ok) throw new Error(j.error || r.statusText);
	return j.name;
}

/** Aufnahmen, die die App verarbeiten kann (wie internal/media.IsMedia) */
export const MEDIA = /\.(mp3|wav|m4a|ogg|opus|flac|aac|wma|mp4|m4v|mov|webm|mkv|avi)$/i;

/**
 * Die Optionen eines Auftrags, wie die Seiten sie zeigen: wie viele Sprecher ('' erkennt die App
 * selbst), welche Sprachen und, wenn beides eingerichtet ist, was erkennt. threads kommt aus den
 * Einstellungen und wird nicht gezeigt. engine: 'local' (die Modelle auf diesem Rechner) oder 'api'
 * (der Dienst aus den Einstellungen); fehlt es, gilt die Vorgabe aus den Einstellungen.
 * @typedef {{ speakers: string | number, language: string, threads?: string | number, engine?: string }} Optionen
 */

/**
 * Die frühere Voreinstellung legte nur die Zahl der Sprecher fest (pipeline/run.go: presets);
 * gespeicherte Vorgaben ohne Sprecherzahl bekommen sie daraus.
 * @param {string} preset @param {number} speakers
 */
export function sprecherAus(preset, speakers) {
	return speakers || ({ interview: 2, vortrag: 1 })[preset] || '';
}

/** Wo erkannt wird (Einstellungen, Erkennung): nur hier, nur extern oder beides */
export const ERKENNUNG = /** @type {const} */ (['local', 'api', 'both']);

/** Die beiden Wege zu erkennen, wie die Seiten sie nennen */
export const ENGINES = /** @type {Record<string, string>} */ ({ local: 'Auf diesem Rechner', api: 'Über die API' });

/**
 * Die Vorgaben aus den Einstellungen als Startwerte für neue Aufträge, dazu, wo erkannt wird.
 * @returns {Promise<Optionen & { dropMode: string, erkennung: string }>}
 */
export async function vorgaben() {
	const p = await getJSON('/api/prefs');
	return {
		speakers: sprecherAus(p.preset, p.speakers),
		language: p.language,
		threads: p.threads,
		engine: p.fileEngine,
		dropMode: p.dropMode,
		erkennung: p.recognition
	};
}

/**
 * Reiht Aufnahmen ein, mit den Werten so, wie die alten Seiten sie schickten (Zahlen als Text,
 * leer heißt automatisch). Gibt die Zahl der neuen Aufträge zurück.
 * @param {string[]} audio
 * @param {Optionen} o
 * @returns {Promise<number>}
 */
export async function einreihen(audio, o) {
	const text = (/** @type {unknown} */ v) => (v === null || v === undefined ? '' : String(v));
	const r = await post('/api/queue/add', {
		audio,
		preset: 'auto',
		speakers: text(o.speakers),
		language: text(o.language).trim() || 'auto',
		threads: text(o.threads),
		engine: o.engine
	});
	return r.added;
}

/**
 * Dauer als m:ss oder h:mm:ss.
 * @param {number} s Sekunden
 */
export function dauer(s) {
	s = Math.max(0, Math.round(s || 0));
	const mm = String(Math.floor(s / 60) % 60).padStart(2, '0');
	const ss = String(s % 60).padStart(2, '0');
	return s >= 3600 ? `${Math.floor(s / 3600)}:${mm}:${ss}` : `${Math.floor(s / 60)}:${ss}`;
}

/**
 * Größe einer Datei: 12,4 MB.
 * @param {number} b Bytes
 */
export function groesse(b) {
	if (b < 1000) return `${b} Byte`;
	const [n, e] = b < 1e6 ? [b / 1e3, 'KB'] : b < 1e9 ? [b / 1e6, 'MB'] : [b / 1e9, 'GB'];
	return `${n.toLocaleString('de-DE', { maximumFractionDigits: n < 10 ? 1 : 0 })} ${e}`;
}

/**
 * Name, Schlagworte und Ausblenden einer Aufnahme (library.json); nur was angegeben ist, ändert
 * sich. Ein leerer Name gibt ihr den Namen ihrer Datei zurück. Gibt den gespeicherten Stand zurück.
 * @param {string} stem
 * @param {{ title?: string, tags?: string[], hidden?: boolean }} was
 * @returns {Promise<{ title: string, tags: string[], hidden: boolean }>}
 */
export async function beschriften(stem, was) {
	const { title, tags, hidden } = await post(`/api/recording?stem=${encodeURIComponent(stem)}`, was);
	return { title, tags, hidden };
}
