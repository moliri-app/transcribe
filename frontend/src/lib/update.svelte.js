// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Ob es eine neuere Version der App gibt und wie weit ihr Update ist (internal/update), für die
// App-Leiste und die Einstellungen. Nach dem Neustart lädt jede offene Seite neu: Die neue EXE
// bringt ihre eigene Oberfläche mit.

import { getJSON, post } from '$lib/api.js';

/** state: '' noch nicht nachgesehen (oder offline), 'current', 'available', 'downloading',
 * 'waiting' (installiert, wartet auf laufende Aufträge), 'restarting', 'failed' (error)
 * @typedef {{ version: string, checks: boolean, state: string, latest?: string, page?: string,
 *   progress?: number, error?: string, checked?: number }} UpdateStand */

export const update = $state({
	/** @type {UpdateStand | null} */
	stand: null,
	/** die App antwortet gerade nicht (sie startet neu) */
	weg: false
});

/** sessionStorage: die Version vor dem Update, für den Hinweis nach dem Neuladen */
const VORHER = 'transcribe-update-vorher';

/** Was nach dem Neuladen zu sagen ist: von welcher Version das Update kam; '' sonst. Einmal. */
export function vorherigeVersion() {
	const v = sessionStorage.getItem(VORHER) ?? '';
	sessionStorage.removeItem(VORHER);
	return v;
}

/** v1.2.3 → 1.2.3; eine Entwicklerversion heißt so @param {string | undefined} v */
export function versionText(v) {
	if (!v) return 'Entwicklerversion';
	return /^v\d/.test(v) ? v.slice(1) : v;
}

/** @type {string | null} die Version, mit der diese Seite geladen wurde */
let geladenMit = null;

export async function ladeUpdate() {
	try {
		const s = /** @type {UpdateStand} */ (await getJSON('/api/update'));
		update.weg = false;
		if (geladenMit === null) geladenMit = s.version;
		else if (s.version !== geladenMit) {
			sessionStorage.setItem(VORHER, geladenMit || 'Entwicklerversion');
			location.reload();
			return;
		}
		update.stand = s;
	} catch {
		update.weg = true;
	}
}

/** gleich wieder nachsehen statt erst beim nächsten Mal (beobachteUpdate) */
let gleich = () => {};

/** @param {'check' | 'install'} action */
export async function updateAktion(action) {
	update.stand = await post('/api/update', { action });
	gleich();
}

/** während eines Updates oft nachsehen, sonst selten */
function bald() {
	const st = update.stand?.state ?? '';
	return update.weg || ['downloading', 'waiting', 'restarting'].includes(st);
}

let laeuft = false;

/** Fragt die App regelmäßig; einmal pro Seite (die App-Leiste startet es). */
export function beobachteUpdate() {
	if (laeuft) return () => {};
	laeuft = true;
	/** @type {ReturnType<typeof setTimeout>} */
	let t;
	const weiter = async () => {
		await ladeUpdate();
		clearTimeout(t);
		t = setTimeout(weiter, bald() ? 1500 : 60_000);
	};
	gleich = () => {
		clearTimeout(t);
		t = setTimeout(weiter, 500);
	};
	weiter();
	return () => {
		clearTimeout(t);
		gleich = () => {};
		laeuft = false;
	};
}
