// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Stand der Live-Seite: die Aufnahmen (Notizen des Servers), das Mikrofon, die Freigaben und die
// Einstellungen fürs Aufnehmen. Den Stand schickt der Server über /api/live/events (internal/live).
import { einreihen, getJSON, post, vorgaben } from '$lib/api.js';
import { langName } from '$lib/moliri/components/files/transcript/editor/sprachen.js';

/** source: woher der Satz kam, wenn die Aufnahme Mikrofon und Systemaudio mischt
 * @typedef {{ text: string, start: number, end: number, source?: 'mic' | 'system' }} Zeile */
/** source: was die App aufgenommen hat; fehlt, wenn ein Programm den Ton geschickt hat. Eine Anfrage an
 * einen Dienst (ocr, filter, embed; internal/live/calls.go) hat statt Sätzen model, request, response und
 * facts; partial sagt, was sie gerade tut.
 * @typedef {{ id: string, kind: 'audio' | 'mic' | 'dauer' | 'diktat' | Dienst, client?: string, started: string, ended?: string,
 *   lines: Zeile[], partial?: string, partialSource?: 'mic' | 'system', audio?: string, language?: string,
 *   source?: Quelle, error?: string, loading?: boolean, waiting?: boolean, transient?: boolean,
 *   model?: string, request?: string, response?: string, facts?: { label: string, value: string }[] }} Notiz */
/** @typedef {keyof typeof DIENSTE} Dienst */
/** @typedef {'mic' | 'system' | 'both'} Quelle */
/** input: was mit dem Mikrofon nicht stimmt, solange die Aufnahme läuft (internal/live/input.go)
 * @typedef {{ running: boolean, source?: Quelle, dauer?: string, listeners: number, input?: 'narrow' | 'quiet' }} Mikrofon */
/** Sprache, die die Erkennung gehört, aber noch nicht als Text hat: so viele Sekunden (pending) zum
 * Zeitpunkt at (Date.now()); solange gesprochen wird, wächst das. Fehlt, solange niemand spricht.
 * @typedef {{ pending: number, at: number }} Gehoert */
/** @typedef {{ origin: string, client?: string, at: string }} Abgelehnt */
/** devices: die angeschlossenen Mikrofone; default: wie das heißt, das im System Standard ist;
 * device: das gewählte, '' für den Standard des Systems
 * @typedef {{ devices: string[], default: string, device: string }} Mikrofone */

/** die Sprachen einer Aufnahme mit Namen: „auto“ oder Codes mit Komma wie „de,en“
 * @param {string | undefined} codes */
export function sprachname(codes) {
	if (!codes || codes === 'auto') return 'Automatisch';
	return codes.split(',').map(langName).join(', ');
}

/** was aufgenommen wird */
export const QUELLEN = { mic: 'Mikrofon', system: 'Systemaudio', both: 'Mikrofon und Systemaudio' };

/** solange die Erkennung bereit ist, aber noch kein Ton kam (ein Bluetooth-Headset braucht dafür einige Sekunden) */
export const WARTET = {
	mic: 'Wartet aufs Mikrofon …',
	system: 'Wartet aufs Systemaudio …',
	both: 'Wartet auf Mikrofon und Systemaudio …'
};

/** was die Live-Seite sagt, wenn das Mikrofon der Erkennung schadet */
export const EINGANG = {
	narrow: 'Dieses Mikrofon liefert nur Telefonqualität, die Erkennung leidet.',
	quiet: 'Dieses Mikrofon ist sehr leise, die Erkennung leidet.'
};

/** wann Anfragen von Programmen gelöscht werden: Stunden nach ihrem Ende, 0 nie */
export const LOESCHEN = /** @type {const} */ ([
	[0, 'Nie, sie bleiben, bis du sie löschst'],
	[1, 'Nach einer Stunde'],
	[24, 'Nach einem Tag'],
	[168, 'Nach einer Woche'],
	[720, 'Nach 30 Tagen']
]);

/** so heißen die Quellen über den Sätzen einer gemischten Aufnahme */
export const LINIE = { mic: 'Mikrofon', system: 'Systemaudio' };

/** die Dienste, die Programme außer der Live-Transkription nutzen */
export const DIENSTE = /** @type {const} */ ({
	ocr: 'Texterkennung',
	filter: 'Filter in eigenen Worten',
	embed: 'Suche nach Bedeutung'
});

/** Eine Anfrage an einen Dienst (Texterkennung, Sprachmodell, …), keine Aufnahme.
 * @param {Notiz} n */
export const anfrage = (n) => n.kind in DIENSTE;

/** Von Hand: auf dieser Seite gestartet oder mit dem Tastenkürzel diktiert. Sonst kam die Anfrage über die
 * API von einem Programm.
 * @param {Notiz} n */
export const vonHand = (n) => n.kind === 'dauer' || n.kind === 'diktat';

/** wer die Aufnahme angestoßen hat
 * @param {Notiz} n */
export function wer(n) {
	if (n.kind === 'diktat') return 'Diktat';
	if (vonHand(n)) return 'Von Hand';
	const c = n.client || 'Programm';
	// die Adresse einer Webseite ohne http://
	return c.replace(/^https?:\/\//, '');
}

/** was das Programm getan hat
 * @param {Notiz} n */
export function art(n) {
	return {
		audio: 'hat Audio geschickt',
		mic: 'hat mithören lassen',
		dauer: 'auf dieser Seite aufgenommen',
		diktat: 'per Tastenkürzel diktiert',
		ocr: 'hat Text in einem Bild lesen lassen',
		filter: 'hat das Sprachmodell gefragt',
		embed: 'hat Texte für die Suche nach Bedeutung umwandeln lassen'
	}[n.kind];
}

/** Worum es bei einer Anfrage ging, in einer Zeile: die Frage, der erste Text, oder was gelesen wurde.
 * @param {Notiz} n */
export function worum(n) {
	const zeilen = (n.request ?? '').split('\n');
	const nach = (/** @type {RegExp} */ muster) => zeilen.find((z) => muster.test(z))?.replace(muster, '') ?? '';
	if (n.kind === 'filter') return nach(/^Frage: /);
	if (n.kind === 'embed') return nach(/^1\. /);
	return (n.response ?? '').replace(/\s+/g, ' ').trim();
}

/** wie lange eine Anfrage dauerte, genauer als die Länge einer Aufnahme: „0,42 s“, „12 s“, „2:05 min“
 * @param {number} s */
export function sekunden(s) {
	if (s < 10) return `${s.toLocaleString('de-DE', { minimumFractionDigits: s < 1 ? 2 : 1, maximumFractionDigits: s < 1 ? 2 : 1 })} s`;
	if (s < 60) return `${Math.round(s)} s`;
	return `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')} min`;
}

/** wie lange die Aufnahme lief, in Sekunden
 * @param {Notiz} n @param {number} jetzt */
export function laenge(n, jetzt) {
	const bis = n.ended ? new Date(n.ended).getTime() : jetzt;
	return Math.max(0, (bis - new Date(n.started).getTime()) / 1000);
}

/** @param {string} iso */
export function uhrzeit(iso) {
	return new Date(iso).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
}

/** „Heute“, „Gestern“ oder der Tag mit Wochentag
 * @param {string} iso @param {number} jetzt */
export function tag(iso, jetzt) {
	const a = new Date(iso);
	const h = new Date(jetzt);
	const tage = Math.round(
		(new Date(h.getFullYear(), h.getMonth(), h.getDate()).getTime() -
			new Date(a.getFullYear(), a.getMonth(), a.getDate()).getTime()) /
			864e5
	);
	if (tage === 0) return 'Heute';
	if (tage === 1) return 'Gestern';
	return a.toLocaleDateString('de-DE', {
		weekday: 'long',
		day: 'numeric',
		month: 'long',
		...(a.getFullYear() !== h.getFullYear() && { year: 'numeric' })
	});
}

/** die Aufnahme als Text; bei gemischter Aufnahme steht vor jedem Wechsel die Quelle (wie Note.Text im Server)
 * @param {Notiz} n */
export function text(n) {
	let quelle = '';
	return n.lines
		.map((l, i) => {
			if (!l.source || l.source === quelle) return l.text;
			quelle = l.source;
			return `${i ? '\n' : ''}${LINIE[l.source]}:\n${l.text}`;
		})
		.map((t, i, alle) => (i < alle.length - 1 && !t.endsWith('\n') ? `${t}\n` : t)) // ein angesagter Umbruch ist schon einer
		.join('');
}

class Live {
	/** @type {Notiz[] | null} */
	notizen = $state(null);
	/** @type {Mikrofon} */
	mic = $state({ running: false, listeners: 0 });
	/** wie laut Mikrofon und Systemaudio gerade sind, 0..1
	 * @type {{ mic?: number, system?: number }} */
	pegel = $state({});
	/** @type {Abgelehnt[]} */
	abgelehnt = $state([]);
	/** @type {string[]} */
	erlaubt = $state([]);
	verbunden = $state(true);
	/** je Aufnahme, was die Erkennung gehört und noch nicht als Text hat
	 * @type {Record<string, Gehoert>} */
	gehoert = $state({});

	/** die angeschlossenen Mikrofone; null, solange sie gesucht werden
	 * @type {string[] | null} */
	geraete = $state(null);
	/** das gewählte Mikrofon; '' nimmt das, das im System Standard ist */
	geraet = $state('');
	/** wie das Mikrofon heißt, das im System Standard ist; '' wenn unbekannt */
	standard = $state('');
	/** @type {Quelle} */
	quelle = $state('mic');
	/** was in der Aufnahme gesprochen wird: auto oder ein Sprachcode */
	sprache = $state('auto');
	/** ob dieser Rechner aufnehmen kann, was er abspielt */
	systemMoeglich = $state(true);
	speichern = $state(true);
	/** „Im Hintergrund bereit“: an dürfen Programme das Mikrofon aufnehmen lassen, und die Erkennung
	 * bleibt geladen; aus werden sie damit abgewiesen. Geschickte Audiodaten und Bilder und von Hand
	 * aufnehmen gehen immer. */
	an = $state(true);
	/** ob Anfragen von Programmen als Aufnahme gespeichert werden */
	apiSpeichern = $state(true);
	/** wie viele Stunden nach ihrem Ende gespeicherte Anfragen gelöscht werden; 0 nie */
	apiLoeschen = $state(0);
	/** ob die Erkennung vorgeladen ist: 'loading', 'ready' oder '' (aus, ein Dienst erkennt, Modelle fehlen noch)
	 * @type {'' | 'loading' | 'ready'} */
	warm = $state('');
	/** „Meine Stimme“: eigene Wörter, die der Live-Text so schreibt, wie sie hier stehen
	 * @type {string[]} */
	woerter = $state([]);
	/** angesagte Satzzeichen („Komma“, „neuer Absatz“) werden zu Zeichen */
	ansagen = $state(true);
	startet = $state(false);
	/** „Beenden“ wurde gedrückt, das Programm erkennt noch, was bis dahin gesagt wurde */
	beendet = $state(false);
	meldung = $state('');
	jetzt = $state(Date.now());

	/** die eigene Aufnahme, die gerade läuft */
	laufend = $derived(this.notizen?.find((n) => n.id === this.mic.dauer) ?? null);

	/** @param {any} m */
	nachricht(m) {
		switch (m.type) {
			case 'snapshot':
				this.notizen = m.notes;
				this.mic = m.mic;
				this.quelle = m.source;
				this.sprache = m.language;
				this.systemMoeglich = m.system;
				this.abgelehnt = m.blocked;
				this.erlaubt = m.allowed;
				this.an = m.enabled;
				this.warm = m.warm;
				this.apiSpeichern = m.apiSave;
				this.apiLoeschen = m.apiKeepHours;
				this.stimme(m.voice);
				// auch nach einer Unterbrechung: was sich derweil an den Geräten geändert hat
				this.mikrofoneLaden();
				break;
			case 'note': {
				if (!this.notizen) break;
				const i = this.notizen.findIndex((n) => n.id === m.note.id);
				if (i >= 0) this.notizen[i] = m.note;
				else this.notizen.push(m.note);
				break;
			}
			case 'line': {
				const n = this.notizen?.find((x) => x.id === m.id);
				if (n) {
					n.lines.push(m.line);
					n.partial = '';
				}
				break;
			}
			case 'partial': {
				const n = this.notizen?.find((x) => x.id === m.id);
				if (n) {
					n.partial = m.text;
					n.partialSource = m.source;
				}
				break;
			}
			case 'hearing':
				if (m.speaking) this.gehoert[m.id] = { pending: m.pending, at: Date.now() };
				else delete this.gehoert[m.id];
				break;
			case 'removed':
				this.notizen = this.notizen?.filter((n) => n.id !== m.id) ?? null;
				break;
			case 'mic':
				this.mic = m.mic;
				if (!m.mic.running) this.pegel = {};
				break;
			case 'level':
				this.pegel = m.levels;
				break;
			case 'access':
				this.abgelehnt = m.blocked;
				this.erlaubt = m.allowed;
				break;
			case 'enabled':
				this.an = m.enabled;
				break;
			case 'warm':
				this.warm = m.warm;
				break;
			case 'apiNotes':
				this.apiSpeichern = m.save;
				this.apiLoeschen = m.keepHours;
				break;
			case 'voice':
				this.stimme(m);
				break;
			case 'devices':
				this.mikrofone(m.microphones);
				break;
		}
	}

	/** @param {{ words: string[], commands: boolean }} v */
	stimme(v) {
		this.woerter = v.words;
		this.ansagen = v.commands;
	}

	/** @param {Mikrofone} m */
	mikrofone(m) {
		this.geraete = m.devices;
		this.standard = m.default;
		this.geraet = m.device;
	}

	mikrofoneLaden() {
		getJSON('/api/live/devices')
			.then((m) => this.mikrofone(m))
			.catch(() => (this.geraete ??= []));
	}

	/** Verbindet mit dem Server; der Stand bringt die Geräte mit. Gibt das Aufräumen zurück. */
	verbinden() {
		/** @type {EventSource} */
		let es;
		const auf = () => {
			es = new EventSource('/api/live/events');
			es.onmessage = (e) => {
				this.verbunden = true;
				this.nachricht(JSON.parse(e.data));
			};
			es.onerror = () => (this.verbunden = false);
		};
		auf();
		try {
			this.speichern = localStorage.getItem('live:speichern') !== '0';
		} catch {
			// ohne Speicher bleibt es beim Speichern
		}
		const uhr = setInterval(() => (this.jetzt = Date.now()), 1000);
		return () => {
			es?.close();
			clearInterval(uhr);
		};
	}

	/** @param {boolean} ja */
	speichernSetzen(ja) {
		this.speichern = ja;
		try {
			localStorage.setItem('live:speichern', ja ? '1' : '0');
		} catch {
			// egal
		}
	}

	/** @param {boolean} an */
	async modusSetzen(an) {
		this.an = an;
		try {
			await post('/api/live/mode', { on: an });
			this.meldung = '';
		} catch (e) {
			this.an = !an;
			this.meldung = /** @type {Error} */ (e).message;
		}
	}

	/** @param {boolean} ja */
	async apiSpeichernSetzen(ja) {
		this.apiSpeichern = ja;
		try {
			await post('/api/live/api-save', { on: ja });
			this.meldung = '';
		} catch (e) {
			this.apiSpeichern = !ja;
			this.meldung = /** @type {Error} */ (e).message;
		}
	}

	/** @param {number} stunden */
	async apiLoeschenSetzen(stunden) {
		const vorher = this.apiLoeschen;
		this.apiLoeschen = stunden;
		try {
			await post('/api/live/api-keep', { keepHours: stunden });
			this.meldung = '';
		} catch (e) {
			this.apiLoeschen = vorher;
			this.meldung = /** @type {Error} */ (e).message;
		}
	}

	async starten() {
		this.startet = true;
		this.meldung = this.notizen?.length || this.warm === 'ready' ? '' : 'Lädt die Erkennung, beim ersten Mal dauert das etwas …';
		try {
			await post('/api/live/start', { save: this.speichern });
			this.meldung = '';
		} catch (e) {
			this.meldung = /** @type {Error} */ (e).message;
		} finally {
			this.startet = false;
		}
	}

	async beenden() {
		if (this.beendet) return;
		this.beendet = true;
		try {
			await post('/api/live/stop', {});
		} catch (e) {
			this.meldung = /** @type {Error} */ (e).message;
		} finally {
			this.beendet = false;
		}
	}

	/** Eine Einstellung ans Programm; sie gilt ab der nächsten Aufnahme.
	 * @param {string} url @param {object} body */
	async #einstellen(url, body) {
		try {
			await post(url, body);
			this.meldung = this.mic.running ? 'Gilt ab der nächsten Aufnahme.' : '';
		} catch (e) {
			this.meldung = /** @type {Error} */ (e).message;
		}
	}
	geraetWaehlen = () => this.#einstellen('/api/live/device', { device: this.geraet });
	spracheWaehlen = () => this.#einstellen('/api/live/language', { language: this.sprache });
	quelleWaehlen = () => this.#einstellen('/api/live/source', { source: this.quelle });

	/** Ein eigenes Wort dazu; gibt zurück, ob es neu war.
	 * @param {string} wort */
	wortDazu(wort) {
		const w = wort.trim().replace(/\s+/g, ' ');
		if (!w || this.woerter.some((x) => x.toLowerCase() === w.toLowerCase())) return false;
		this.woerter = [...this.woerter, w];
		this.#einstellen('/api/live/words', { words: this.woerter });
		return true;
	}

	/** @param {string} wort */
	wortWeg(wort) {
		this.woerter = this.woerter.filter((x) => x !== wort);
		this.#einstellen('/api/live/words', { words: this.woerter });
	}

	/** @param {boolean} an */
	ansagenSetzen(an) {
		this.ansagen = an;
		this.#einstellen('/api/live/commands', { on: an });
	}

	/** @param {Notiz} n */
	async loeschen(n) {
		try {
			await post('/api/live/delete', { id: n.id });
		} catch (e) {
			this.meldung = /** @type {Error} */ (e).message;
		}
	}

	/** Die gespeicherte Aufnahme vollständig transkribieren lassen, in der Sprache, die für sie gewählt war;
	 * gibt zurück, was dabei herauskam.
	 * @param {Notiz} n */
	async nachtranskribieren(n) {
		if (!n.audio) return '';
		try {
			const { dropMode: _, ...o } = await vorgaben();
			const k = await einreihen([n.audio], { ...o, language: n.language ?? o.language });
			return k ? 'Wird transkribiert, danach steht sie in der Bibliothek.' : 'Wird schon transkribiert.';
		} catch (e) {
			return /** @type {Error} */ (e).message;
		}
	}

	/** @param {string} origin @param {boolean} ja */
	async freigeben(origin, ja) {
		try {
			await post(ja ? '/api/live/allow' : '/api/live/forbid', { origin });
		} catch (e) {
			this.meldung = /** @type {Error} */ (e).message;
		}
	}
}

export const live = new Live();
