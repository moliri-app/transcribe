// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Stand der Live-Seite: die Aufnahmen (Notizen des Servers), das Mikrofon, die Freigaben und die
// Einstellungen fürs Aufnehmen. Den Stand schickt der Server über /api/live/events (internal/live).
import { einreihen, getJSON, post, vorgaben } from '$lib/api.js';
import { langName } from '$lib/moliri/components/files/transcript/editor/sprachen.js';
import { getLocale } from '$lib/moliri/i18n/locale.js';
import { argumente, leserName, meldung } from '$lib/meldungen.js';
import plural from '../../locales/plural.js';

/** source: woher der Satz kam, wenn die Aufnahme Mikrofon und Systemaudio mischt
 * @typedef {{ text: string, start: number, end: number, source?: 'mic' | 'system' }} Zeile */
/** source: was die App aufgenommen hat; fehlt, wenn ein Programm den Ton geschickt hat. Eine Anfrage an
 * einen Dienst (ocr, filter, embed; internal/live/calls.go) hat statt Sätzen model, request, response und
 * facts (bei der Texterkennung auch das Bild, image); partial sagt, was sie gerade tut.
 * @typedef {{ id: string, kind: 'audio' | 'mic' | 'dauer' | 'diktat' | Dienst, client?: string, started: string, ended?: string,
 *   lines: Zeile[], partial?: string, partialMsg?: Meldung, partialSource?: 'mic' | 'system', audio?: string, language?: string,
 *   source?: Quelle, error?: string, errorMsg?: Meldung, loading?: boolean, waiting?: boolean, transient?: boolean,
 *   model?: string, request?: string, response?: string, facts?: Fakt[], image?: string }} Notiz */
/** label und value auf Deutsch, msg dasselbe zum Übersetzen (fakt)
 * @typedef {{ label: string, value: string, msg?: Meldung }} Fakt */
/** @typedef {import('$lib/meldungen.js').Meldung} Meldung */
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
	if (!codes || codes === 'auto') return /* @wc-include */ 'Automatic';
	return codes.split(',').map(langName).join(', ');
}

// The tables of texts below are getters, so each read takes the text in the interface language.

/** was aufgenommen wird */
export const QUELLEN = {
	get mic() {
		return /* @wc-include */ 'Microphone';
	},
	get system() {
		return /* @wc-include */ 'System audio';
	},
	get both() {
		return /* @wc-include */ 'Microphone and system audio';
	}
};

/** solange die Erkennung bereit ist, aber noch kein Ton kam (ein Bluetooth-Headset braucht dafür einige Sekunden) */
export const WARTET = {
	get mic() {
		return /* @wc-include */ 'Waiting for the microphone…';
	},
	get system() {
		return /* @wc-include */ 'Waiting for system audio…';
	},
	get both() {
		return /* @wc-include */ 'Waiting for microphone and system audio…';
	}
};

/** was die Live-Seite sagt, wenn das Mikrofon der Erkennung schadet */
export const EINGANG = {
	get narrow() {
		return /* @wc-include */ 'This microphone only delivers phone quality, so recognition suffers.';
	},
	get quiet() {
		return /* @wc-include */ 'This microphone is very quiet, so recognition suffers.';
	}
};

/** wann Anfragen von Programmen gelöscht werden: Stunden nach ihrem Ende, 0 nie */
export const LOESCHEN = [
	wahl(0, () => /* @wc-include */ 'Never, they stay until you delete them'),
	wahl(1, () => /* @wc-include */ 'After one hour'),
	wahl(24, () => /* @wc-include */ 'After one day'),
	wahl(168, () => /* @wc-include */ 'After one week'),
	wahl(720, () => /* @wc-include */ 'After 30 days')
];

/** an entry [hours, name] whose name is read in the interface language each time
 * @param {number} stunden @param {() => string} name @returns {readonly [number, string]} */
function wahl(stunden, name) {
	return /** @type {any} */ (Object.defineProperty([stunden], 1, { get: name, enumerable: true }));
}

/** so heißen die Quellen über den Sätzen einer gemischten Aufnahme */
export const LINIE = {
	get mic() {
		return /* @wc-include */ 'Microphone';
	},
	get system() {
		return /* @wc-include */ 'System audio';
	}
};

/** die Dienste, die Programme außer der Live-Transkription nutzen */
export const DIENSTE = {
	get ocr() {
		return /* @wc-include */ 'Text recognition';
	},
	get filter() {
		return /* @wc-include */ 'Filter in your own words';
	},
	get embed() {
		return /* @wc-include */ 'Search by meaning';
	}
};

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
	if (n.kind === 'diktat') return /* @wc-context: live_who */ /* @wc-include */ 'Dictation';
	if (vonHand(n)) return /* @wc-include */ 'Manual';
	const c = n.client || /* @wc-include */ 'App';
	// die Adresse einer Webseite ohne http://
	return c.replace(/^https?:\/\//, '');
}

/** was das Programm getan hat
 * @param {Notiz} n */
export function art(n) {
	return {
		audio: /* @wc-include */ 'sent audio',
		mic: /* @wc-include */ 'let the app listen in',
		dauer: /* @wc-include */ 'recorded on this page',
		diktat: /* @wc-include */ 'dictated with the keyboard shortcut',
		ocr: /* @wc-include */ 'had text in an image read',
		filter: /* @wc-include */ 'asked the language model',
		embed: /* @wc-include */ 'had texts converted for search by meaning'
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

/** Was eine Anfrage beantwortete, in der Sprache der Oberfläche (die Texterkennung von Windows, ein KI-Dienst).
 * @param {Notiz} n */
export const modell = (n) => (n.kind === 'ocr' ? leserName(n.model ?? '') : (n.model ?? ''));

/** Der Fehler einer Notiz in der Sprache der Oberfläche (der Server schickt ihn auf Deutsch und als
 * Meldung, internal/msg).
 * @param {Notiz} n */
export const fehler = (n) => meldung(n.errorMsg, n.error ?? '');

/** Was gerade erkannt wird, oder bei einer Anfrage, was sie gerade tut (eine Meldung des Servers).
 * @param {Notiz} n */
export const zwischenstand = (n) => (n.partialMsg ? meldung(n.partialMsg, n.partial ?? '') : (n.partial ?? ''));

/** Ein Fakt einer Anfrage (internal/live/calls.go: Fact) mit Namen und Wert in der Sprache der
 * Oberfläche: die Meldung nennt mit ihrem Schlüssel den Fakt, ihre Argumente machen den Wert.
 * @param {Fakt} f
 * @returns {{ label: string, value: string }} */
export function fakt(f) {
	const m = f.msg;
	if (!m) return { label: f.label, value: f.value };
	const a = argumente(m);
	/** @type {Record<string, number>} */
	const zahl = new Proxy({}, { get: (_, k) => Number(m.args?.[String(k)] ?? 0) });
	switch (m.key) {
		case 'fact.recognized':
			return {
				label: /* @wc-include */ 'Recognized',
				value: `${plural(zahl.lines, ['# line', '# lines'])}, ${plural(zahl.words, ['# word', '# words'])}`
			};
		case 'fact.engine':
			return { label: /* @wc-include */ 'Engine', value: a.name };
		case 'fact.imageSize':
			return { label: /* @wc-include */ 'Image size', value: /* @wc-include */ `${a.width} × ${a.height} pixels` };
		case 'fact.models':
			return { label: /* @wc-include */ 'Models', value: a.models };
		case 'fact.gpu':
			return { label: /* @wc-include */ 'Graphics card', value: /* @wc-include */ 'yes' };
		case 'fact.texts':
			return { label: /* @wc-include */ 'Texts', value: /* @wc-include */ `${a.texts}, ${a.chars} characters in all` };
		case 'fact.model':
			return {
				label: /* @wc-include */ 'Model',
				value: m.args?.loaded ? /* @wc-include */ 'was already loaded' : /* @wc-include */ 'had to be loaded first'
			};
		case 'fact.waited':
			return { label: /* @wc-include */ 'Waited', value: sekunden(zahl.seconds) };
		case 'fact.computeTime':
			return { label: /* @wc-include */ 'Computing time', value: sekunden(zahl.seconds) };
		case 'fact.tokensRead':
			return { label: /* @wc-include */ 'Tokens read', value: /* @wc-include */ `${a.tokens}, ${a.cached} of them already cached` };
		case 'fact.tokensWritten':
			return { label: /* @wc-include */ 'Tokens written', value: a.tokens };
		case 'fact.device':
			return { label: /* @wc-include */ 'Device', value: a.device };
		case 'fact.result':
			return {
				label: /* @wc-include */ 'Result',
				value: /* @wc-include */ `${plural(zahl.conditions, ['# condition', '# conditions'])}, ${a.notUnderstood} not understood, ${a.dropped} dropped`
			};
	}
	return { label: f.label, value: f.value };
}

/** Die Anfrage, wie die Live-Seite sie zeigt: die Zeilen, mit denen der Server sie beschreibt (Bild,
 * Sprachen, Frage …, internal/server: ocrRequest, filterRequest, embedRequest), in der Sprache der
 * Oberfläche; was danach kommt (die Texte, die Nachrichten an das Sprachmodell), wie es ist.
 * @param {Notiz} n */
export function anfrageText(n) {
	const r = n.request ?? '';
	const ende = r.indexOf('\n\n');
	const kopf = ende < 0 ? r : r.slice(0, ende);
	/** "Standard" und "ohne Angabe" des Servers */
	const wert = (/** @type {string} */ v) =>
		v === 'Standard' ? /* @wc-include */ 'default' : v === 'ohne Angabe' ? /* @wc-include */ 'not given' : v;
	const zeilen = kopf.split('\n').map((z) => {
		const [, name, v] = /^(Bild|Sprachen|Layout|Frage|Felder|Art): (.*)$/.exec(z) ?? [];
		switch (name) {
			case 'Bild':
				return /* @wc-include */ `Image: ${wert(v)}`;
			case 'Sprachen':
				return /* @wc-include */ `Languages: ${wert(v)}`;
			case 'Layout':
				return /* @wc-include */ `Layout: ${wert(v)}`;
			case 'Frage':
				return /* @wc-include */ `Question: ${v}`;
			case 'Felder':
				return /* @wc-include */ `Fields: ${v}`;
			case 'Art':
				return /* @wc-include */ `Kind: ${v}`;
		}
		return z;
	});
	return gekuerzt(zeilen.join('\n') + (ende < 0 ? '' : r.slice(ende)));
}

/** Die Antwort, wie die Live-Seite sie zeigt; bei der Suche nach Bedeutung die erste Zeile (wie viele
 * Vektoren) in der Sprache der Oberfläche.
 * @param {Notiz} n */
export function antwortText(n) {
	const r = n.response ?? '';
	const v = /^(\d+) Vektoren mit je (\d+) Zahlen/.exec(r);
	if (n.kind !== 'embed' || !v) return gekuerzt(r);
	return gekuerzt(/* @wc-include */ `${v[1]} vectors of ${v[2]} numbers each` + r.slice(v[0].length));
}

/** was der Server an eine gekürzte Anfrage oder Antwort hängt (internal/live/calls.go: clip)
 * @param {string} s */
function gekuerzt(s) {
	const k = /\n… \(gekürzt, insgesamt (\d+) Zeichen\)$/.exec(s);
	if (!k) return s;
	return s.slice(0, k.index) + '\n' + /* @wc-include */ `… (shortened, ${Number(k[1]).toLocaleString(getLocale())} characters in all)`;
}

/** wie lange eine Anfrage dauerte, genauer als die Länge einer Aufnahme: „0,42 s“, „12 s“, „2:05 min“
 * @param {number} s */
export function sekunden(s) {
	const stellen = s < 1 ? 2 : 1;
	const kurz = s < 10 ? s.toLocaleString(getLocale(), { minimumFractionDigits: stellen, maximumFractionDigits: stellen }) : Math.round(s);
	if (s < 60) return /* @wc-include */ `${kurz} s`;
	const min = Math.floor(s / 60);
	const sek = String(Math.floor(s % 60)).padStart(2, '0');
	return /* @wc-include */ `${min}:${sek} min`;
}

/** wie lange die Aufnahme lief, in Sekunden
 * @param {Notiz} n @param {number} jetzt */
export function laenge(n, jetzt) {
	const bis = n.ended ? new Date(n.ended).getTime() : jetzt;
	return Math.max(0, (bis - new Date(n.started).getTime()) / 1000);
}

/** @param {string} iso */
export function uhrzeit(iso) {
	return new Date(iso).toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' });
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
	if (tage === 0) return /* @wc-include */ 'Today';
	if (tage === 1) return /* @wc-include */ 'Yesterday';
	return a.toLocaleDateString(getLocale(), {
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
					n.partialMsg = m.msg;
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
		this.meldung = this.notizen?.length || this.warm === 'ready' ? '' : /* @wc-include */ 'Loading speech recognition, the first time takes a moment…';
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
			this.meldung = this.mic.running ? /* @wc-include */ 'Applies from the next recording on.' : '';
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
			return k
				? /* @wc-include */ 'Being transcribed, then it will be in the library.'
				: /* @wc-include */ 'Already being transcribed.';
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
