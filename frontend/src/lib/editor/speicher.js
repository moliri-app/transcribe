// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Der Speicher des Transkript-Editors in Transcribe: der Editor ist aus Moliri gespiegelt
// ($lib/moliri/components/files/transcript/editor/), woher die Daten kommen und wohin gespeichert
// wird, sagt dieser Speicher (`Speicher` in editor.svelte.js). Alles über die HTTP-API der EXE:
// Transkript und Korrekturen aus results/<stem>/, Speichern über /api/annotations, Wellenform,
// Vorschaubilder, Export, Clips und „Neu berechnen“ über /api/apply. Ohne Versionsstand: es gibt nur
// diesen einen Nutzer. Sprache und Zusammenlegen rechnet erst „Neu berechnen“, deshalb `recompute`.

/** @typedef {{ name: string, stem: string, url: string, video: boolean, engines: { final?: boolean }[] }} Datei */

/** Name einer Aufnahme ohne Endung, wie in der Auswahl */
export const recordingName = (/** @type {Datei} */ f) => f.name.replace(/\.[^.]+$/, '').replace(/_/g, ' ');

/** Gemerktes im Browser (Präfix ed:, wie bisher); im privaten Fenster eben nicht */
export const merken = {
	get(/** @type {string} */ k) {
		try {
			return localStorage.getItem('ed:' + k);
		} catch {
			return null;
		}
	},
	set(/** @type {string} */ k, /** @type {string | null} */ v) {
		try {
			if (v === null) localStorage.removeItem('ed:' + k);
			else localStorage.setItem('ed:' + k, v);
		} catch {
			// nicht merken
		}
	}
};

const resultsPath = (/** @type {string} */ stem) => `/results/${stem.split('/').map(encodeURIComponent).join('/')}`;

/** Fehler einer Anfrage als Satzteil für die Statuszeile */
const reason = (/** @type {unknown} */ err) => {
	const msg = err instanceof Error ? err.message : String(err);
	return msg === 'Failed to fetch' ? 'Transcribe antwortet nicht' : msg;
};

/** Antwort als Datei herunterladen, unter dem Namen aus Content-Disposition; gibt den Namen zurück */
async function download(/** @type {Response} */ r, /** @type {string} */ fallback) {
	const match = /filename\*=UTF-8''([^;]+)/.exec(r.headers.get('Content-Disposition') || '');
	const a = document.createElement('a');
	a.href = URL.createObjectURL(await r.blob());
	a.download = match ? decodeURIComponent(match[1]) : fallback;
	document.body.appendChild(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(a.href), 60000);
	return a.download;
}

/** GET, der bei einer Ablehnung mit dem Text des Servers wirft */
async function fetchOk(/** @type {string} */ url) {
	const r = await fetch(url);
	if (!r.ok) throw new Error((await r.text()).trim() || String(r.status));
	return r;
}

/**
 * Lädt, was der Editor zum Öffnen einer Aufnahme braucht (Editor.load).
 * @param {Datei} f
 */
export async function loadRecording(f) {
	const path = resultsPath(f.stem);
	const [final, annotations] = await Promise.all([
		fetch(`${path}/_final.json`).then((r) => {
			if (!r.ok) throw new Error(String(r.status));
			return r.json();
		}),
		fetch(`${path}/_annotations.json`)
			.then((r) => (r.ok ? r.json() : {}))
			.catch(() => ({}))
	]);
	return { final, annotations, version: 0, isVideo: !!f.video, mediaUrl: '/' + f.url, canEdit: true };
}

/**
 * @param {string} stem die Aufnahme (Ordner unter results/)
 * @returns {import('$lib/moliri/components/files/transcript/editor/editor.svelte.js').Speicher}
 */
export function transcribeStore(stem) {
	const q = encodeURIComponent(stem);

	return {
		async save(ann) {
			// Aufnahme und Korrekturen gehen zusammen weg; der Server prüft, dass sie zusammengehören.
			// Kurz genug, geht es auch noch beim Verlassen der Seite weg (keepalive trägt bis 64 KB).
			const body = JSON.stringify({ ...ann, stem });
			try {
				const r = await fetch(`/api/annotations?stem=${q}`, { method: 'POST', body, keepalive: body.length < 60000 });
				if (r.ok) return { status: 'saved' };
				return { status: r.status >= 400 && r.status < 500 ? 'refused' : 'error' };
			} catch {
				return { status: 'error' };
			}
		},

		async peaks() {
			try {
				const r = await fetch(`/api/peaks?stem=${q}`);
				return r.ok ? await r.json() : null;
			} catch {
				return null;
			}
		},

		async thumbs() {
			const r = await fetch(`/api/thumbs?stem=${q}`);
			if (!r.ok) return null;
			const meta = await r.json();
			return { meta, url: `/${meta.file.split('/').map(encodeURIComponent).join('/')}?v=${meta.count}` };
		},

		async exportTranscript(format) {
			try {
				await download(await fetchOk(`/api/export?stem=${q}&format=${format}`), `transkript.${format}`);
				return null;
			} catch (err) {
				return reason(err);
			}
		},

		async exportClips(clips, format) {
			const ids = encodeURIComponent(clips.map((c) => c.id).join(','));
			try {
				await download(await fetchOk(`/api/clips/export?stem=${q}&ids=${ids}&format=${format}`), `clip.${format}`);
				return null;
			} catch (err) {
				return reason(err);
			}
		},

		recompute: {
			async run() {
				try {
					const r = await fetch(`/api/apply?stem=${q}`, { method: 'POST' });
					/** @type {{ ok?: boolean, error?: string, final?: any }} */
					const j = await r.json().catch(() => ({}));
					if (!r.ok || !j.ok || !j.final) return { error: j.error || r.statusText || `Fehler ${r.status}` };
					return { final: j.final };
				} catch (err) {
					return { error: reason(err) };
				}
			},
			// je Aufnahme gemerkt, unter demselben Schlüssel wie früher
			due: () => merken.get('apply:' + stem) === '1',
			mark: (on) => merken.set('apply:' + stem, on ? '1' : null)
		}
	};
}
