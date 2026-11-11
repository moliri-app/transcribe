// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/editor.svelte.js) von frontend/scripts/sync-moliri.mjs.
// Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri.

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Ported from transcribe-go (frontend/src/lib/editor/editor.svelte.js). Zustand des Editors:
// Transkript, Korrekturen, Sprecherfilter und das Abspielen. Die Teile (Zeitleiste, Manuskript,
// Seitenspalte, Player) lesen und ändern ihn über eine Instanz.
//
// Woher die Daten kommen und wohin gespeichert wird, sagt ein Speicher (`store`, in Moliri
// moliriStore.js): Laden, Speichern der Korrekturen mit Versionsstand, Wellenform, Vorschaubilder,
// Export. Moliris Speicher wendet jede Korrektur gleich an (Zusammenlegen, Namen) und lässt Stellen
// mit festgelegter Sprache von selbst neu erkennen. Ein Speicher, der das nicht kann (transcribe-go),
// bringt `recompute` mit: dann merkt der Editor Sprache und Zusammenlegen für „Neu berechnen“ vor.
//
// Der Editor wird nach transcribe-go gespiegelt (dort frontend/scripts/sync-moliri.mjs): Dateien in
// diesem Ordner importieren nur einander und Paraglide, nichts sonst aus Moliri.

import * as m from '$lib/moliri/texte.js';
import { getLocale } from '$lib/moliri/locale.js';
import { buildBlocks, buildItems, buildWords, defaultTitle, playRanges, r3, rangeText, speakerIds } from './model.js';
import { recognizable } from './sprachen.js';

/** @typedef {import('./model.js').Annotationen} Annotationen */

/**
 * @typedef {object} Speicher
 * @property {(ann: Annotationen, version: number) => Promise<{ status: 'saved' | 'unchanged' | 'conflict' | 'error' | 'refused', final?: any, version?: number, annotations?: any, applyFailed?: boolean, languagesPending?: number }>} save
 * @property {() => Promise<{ peaks: number[], step: number, duration: number } | null>} peaks
 * @property {() => Promise<{ meta: { count: number, step: number, cols: number, rows: number, w: number, h: number }, url: string } | null>} thumbs
 * @property {(format: string) => Promise<string | null>} exportTranscript gibt eine Fehlermeldung zurück, sonst null
 * @property {(clips: import('./model.js').Clip[], format: string, status: (text: string) => void) => Promise<string | null>} exportClips
 * @property {Neuberechnen} [recompute] nur, wo der Server Sprache und Zusammenlegen nicht von selbst rechnet
 */

/**
 * „Neu berechnen“: run rechnet das Transkript aus den gespeicherten Korrekturen neu (das neue final
 * oder eine Fehlermeldung); due und mark merken je Aufnahme, ob etwas vorgemerkt ist, auch über ein
 * Neuladen der Seite hinweg.
 * @typedef {object} Neuberechnen
 * @property {() => Promise<{ final?: any, error?: string }>} run
 * @property {() => boolean} due
 * @property {(on: boolean) => void} mark
 */

/** was nur „Neu berechnen“ rechnet: Sprachen, die der Server neu erkennen kann (je Sprecher und Bereich), und Zusammenlegen */
const serverPart = (/** @type {Annotationen} */ a) =>
	JSON.stringify([
		Object.entries(a.speakers)
			.map(([k, v]) => [k, v.language && recognizable(v.language) ? v.language : '', v.merge_into ?? ''])
			.filter(([, l, mi]) => l || mi),
		a.spans.filter((s) => s.language && recognizable(s.language)).map((s) => [s.start, s.end, s.language])
	]);

const clock = () => new Date().toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' });

/** Farbe eines Sprechers (acht Töne, siehe --spk0 … --spk7 des Editors) */
export const spkColor = (/** @type {number | string} */ k) => `var(--spk${+k % 8})`;

export class Editor {
	/** @type {any} */ final = $state.raw(null);
	/** @type {Annotationen} */ ann = $state.raw({ speakers: {}, spans: [], edits: [], clips: [] });
	/** @type {{ peaks: number[], step: number, duration: number } | null} */ peaks = $state.raw(null);
	/** 'laden' | 'bereit' */
	phase = $state('laden');

	/** darf korrigieren (sonst nur lesen, abspielen, exportieren) */
	canEdit = $state(false);
	isVideo = $state(false);
	mediaUrl = '';
	/** Stellen mit festgelegter Sprache werden gerade neu erkannt (Auftrag in der Text-Spur) */
	languagesBusy = $state(false);
	/** wenig Platz (im Fenster der Datei, nicht im Vollbild): Zeitleiste und Kopf rücken zusammen */
	compact = $state(true);
	/** die Modelle, mit denen transkribiert wurde (CC BY 4.0 verlangt ihre Nennung) */
	attribution = $state('');

	/** Wörter mit allen Korrekturen, Zeitanker, Einträge des Texts */
	/** @type {import('./model.js').Wort[]} */ words = $state.raw([]);
	/** @type {number[]} */ anchors = $state.raw([]);
	/** @type {import('./model.js').Eintrag[]} */ items = $state.raw([]);

	/** ausgeblendete Sprecher; aufgeklappte Stellen (Zeitbereiche), die wieder zu sehen und zu hören sind */
	/** @type {Set<number>} */ hidden = $state.raw(new Set());
	/** @type {{a: number, b: number}[]} */ opened = $state.raw([]);

	/** Text und Zeitleiste laufen beim Abspielen mit */
	follow = $state(true);
	rate = $state(1);
	playing = $state(false);
	time = $state(0);
	mediaDuration = $state(0);
	/** Vorschaubilder eines Videos: { meta, img, url } */
	/** @type {{ meta: { count: number, step: number, cols: number, rows: number, w: number, h: number }, img: HTMLImageElement, url: string } | null} */
	thumbs = $state.raw(null);

	/** Zeitleiste: sichtbarer Ausschnitt in Sekunden; Bereich unter der Maus (Wort im Text) */
	view = $state.raw({ a: 0, b: 60 });
	/** @type {{ a: number, b: number } | null} */ hover = $state.raw(null);

	/** Stelle im Text, die zuletzt angeklickt wurde: Abspielen beginnt dort; Wortleiste (i, own = von ihr gestartet) */
	/** @type {number | null} */ caretT = null;
	/** @type {{ i: number, own: boolean } | null} */ wordbar = $state(null);
	/** offenes Sprechermenü, am Element, das es geöffnet hat */
	/** @type {{ k: number, anchor: HTMLElement } | null} */ speakerMenu = $state.raw(null);
	/** Text-Scrollen durch den Nutzer: Mitlaufen wartet bis dahin */
	userScroll = 0;

	/** Suche im Text: Einträge der Treffer und der gerade gezeigte */
	/** @type {Set<number>} */ hits = $state.raw(new Set());
	/** @type {{ from: number, to: number } | null} */ hitNow = $state.raw(null);

	/** @type {HTMLVideoElement | null} */ media = null;
	#raf = 0;

	/** Korrekturen, die schon in final stecken */
	appliedN = 0;
	/** zählt jedes Neuzeichnen des Texts: der Text ist direkt editierbar, getippte Absätze werden danach frisch gebaut */
	textGen = $state(0);

	/** Speichern: Stand für die Statuszeile; 'ok' gespeichert, 'offen' wartet oder getippt, 'fehler' */
	saveText = $state('');
	saveKind = $state('ok');
	/** Versionsstand der gespeicherten Korrekturen, auf dem diese beruhen */
	version = 0;
	#dirty = false;
	/** @type {ReturnType<typeof setTimeout> | undefined} */ #saveTimer;
	/** @type {Promise<boolean> | null} */ #saving = null;

	/** Rückgängig: frühere Stände der Korrekturen */
	/** @type {string[]} */ #undo = [];
	canUndo = $state(false);
	/** „Neu berechnen“ (nur mit store.recompute): Sprache oder Zusammenlegen ist vorgemerkt; rechnet gerade */
	pending = $state(false);
	computing = $state(false);
	/** Stand von Sprache und Zusammenlegen, den final schon kennt; null, wenn unbekannt (vorgemerkt beim Laden) */
	/** @type {string | null} */ #computed = null;

	/** ein getippter Absatz ist offen; das Manuskript übernimmt (commit) oder verwirft ihn */
	editing = $state(false);
	/** @type {{ commit: () => boolean, discard: () => void } | null} */ textEdit = null;

	/** markierter Bereich (Text oder Zeitleiste) mit dem Dialog daneben; Schleife; Halt am Ende eines Bereichs */
	/** @type {{ a: number, b: number, x: number, y: number } | null} */ range = $state.raw(null);
	/** @type {{ a: number, b: number } | null} */ sel = $state.raw(null);
	/** @type {{ a: number, b: number } | null} */ loop = $state.raw(null);
	/** @type {number | null} */ stopAt = null;
	/** Zeitanker, der gerade gezogen wird (Wortnummer und Zeit), für Text und Zeitleiste */
	/** @type {{ i: number, t: number } | null} */ anchorDrag = $state.raw(null);

	/** Clips: Dialog (anlegen, ändern, exportieren) an der Stelle x, y; abgehakt in der Seitenspalte; Filter nach Schlagwort; zuletzt gezeigter Clip */
	/** @type {{ kind: 'clip', clip: { id?: string, start: number, end: number, title?: string, tags?: string[] }, x: number, y: number } | { kind: 'export', list: import('./model.js').Clip[], x: number, y: number } | null} */
	clipDlg = $state.raw(null);
	/** @type {Set<string>} */ clipSel = $state.raw(new Set());
	/** @type {string | null} */ clipTag = $state(null);
	/** @type {string | null} */ activeClip = $state(null);
	/** Clips, die nacheinander laufen (zeitlich sortiert), und welcher gerade dran ist */
	/** @type {{ a: number, b: number, id: string }[] | null} */ playList = $state.raw(null);
	playIdx = $state(0);
	/** Stelle, die der Text zeigen soll (der Zähler macht jeden Wunsch neu) */
	/** @type {{ t: number, n: number } | null} */ revealT = $state.raw(null);

	speakers = $derived(speakerIds(this.words));
	text = $derived(buildBlocks(this.items, this.hidden, (t) => this.isOpened(t)));
	playable = $derived(playRanges(this.words, this.hidden, this.text.groups));
	/** ein einziger sichtbarer Sprecher von mehreren: er hat den Fokus */
	focused = $derived.by(() => {
		const vis = this.speakers.filter((k) => !this.hidden.has(k));
		return this.speakers.length > 1 && vis.length === 1 ? vis[0] : null;
	});
	nCorrections = $derived(this.ann.edits.length + this.ann.spans.length + Object.keys(this.ann.speakers).length);
	/** der Clip, der gerade läuft (nur dieser oder nacheinander) */
	playingClip = $derived(this.playing ? (this.playList?.[this.playIdx]?.id ?? null) : null);
	/** alle Schlagworte der Clips, alphabetisch */
	allTags = $derived([...new Set(this.ann.clips.flatMap((c) => c.tags || []))].sort((x, y) => x.localeCompare(y, getLocale())));

	/** @param {Speicher} store */
	constructor(store) {
		/** @type {Speicher} */
		this.store = store;
	}

	/** der Speicher braucht „Neu berechnen“ für Sprache und Zusammenlegen (sonst wirken sie von selbst) */
	get recomputes() {
		return !!this.store.recompute;
	}

	duration() {
		return this.mediaDuration || this.final?.duration || this.peaks?.duration || 1;
	}

	/** Name eines Sprechers: von Hand gesetzt, sonst der Maschine (ohne deren Platzhalter S1, S2 …), sonst „Sprecher n“ */
	spkName(/** @type {number | string} */ k) {
		const own = this.ann.speakers[k]?.name;
		if (own) return own;
		const given = String(this.final?.speakers?.[k]?.name ?? '');
		return given && given !== `S${+k + 1}` ? given : m.transcript_speaker({ n: +k + 1 });
	}

	// ---------------- laden ----------------

	/**
	 * Ein Transkript zum Bearbeiten. Mit demselben Stand der Korrekturen (Neuladen nach dem Speichern
	 * oder einem Neulauf der Maschine) bleiben Ansicht, Rückgängig und Abspielstelle.
	 * @param {{ final: any, annotations: any, version: number, isVideo: boolean, mediaUrl: string, canEdit: boolean, attribution?: string }} data
	 */
	load(data) {
		const first = this.phase !== 'bereit';
		// gerade getippt oder noch nicht gespeichert: das bleibt, nur die Maschine kommt neu
		// ebenso, wenn die Antwort älter ist als das zuletzt Gespeicherte
		const keep = !first && (this.#dirty || !!this.#saving || this.editing || (Number(data.version) || 0) < this.version);
		this.final = data.final;
		this.canEdit = data.canEdit;
		this.isVideo = data.isVideo;
		this.attribution = data.attribution ?? '';
		if (!keep) {
			const an = data.annotations || {};
			this.ann = { speakers: an.speakers && !Array.isArray(an.speakers) ? an.speakers : {}, spans: an.spans || [], edits: an.edits || [], clips: an.clips || [] };
			this.version = Number(data.version) || 0;
			if (this.store.recompute) {
				this.pending = this.store.recompute.due();
				this.#computed = this.pending ? null : serverPart(this.ann);
			}
		}
		if (first) {
			this.mediaUrl = data.mediaUrl;
			if (this.media) {
				this.media.src = data.mediaUrl;
				this.media.playbackRate = this.rate;
			}
			this.saveKind = 'ok';
			this.saveText = this.nCorrections ? m.te_saved_all() : m.te_no_corrections();
			this.view = { a: 0, b: Math.min(data.final?.duration || 120, 120) };
			this.store.peaks().then((pk) => (this.peaks = pk));
			if (data.isVideo) this.loadThumbs();
		}
		this.rebuild();
		this.phase = 'bereit';
	}

	rebuild() {
		const { words, anchors, appliedN } = buildWords(this.final, this.ann);
		this.words = words;
		this.anchors = anchors;
		this.appliedN = appliedN;
		this.items = buildItems(words, this.final.events || []);
		this.textGen++;
	}

	// ---------------- Korrigieren, Speichern, Rückgängig ----------------

	/**
	 * Ändert die Korrekturen: merkt den Stand für Rückgängig, wendet fn auf eine Kopie an, zeichnet
	 * neu und speichert gleich danach. Ein offener getippter Absatz wird vorher übernommen.
	 * @param {(ann: Annotationen) => void} fn
	 */
	change(fn) {
		if (!this.canEdit) return;
		this.textEdit?.commit();
		const before = JSON.stringify(this.ann);
		const next = JSON.parse(before);
		fn(next);
		if (JSON.stringify(next) === before) return;
		this.#remember(before);
		this.#apply(next);
	}

	#remember(/** @type {string} */ state) {
		this.#undo.push(state);
		if (this.#undo.length > 100) this.#undo.shift();
		this.canUndo = true;
	}
	#apply(/** @type {Annotationen} */ next) {
		// zurückgenommen, was noch nicht gerechnet war: dann ist auch nichts mehr vorgemerkt
		if (this.store.recompute) {
			const part = serverPart(next);
			if (part !== serverPart(this.ann)) this.#setPending(this.#computed === null || part !== this.#computed);
		}
		this.ann = next;
		this.wordbar = null;
		this.rebuild();
		this.#dirty = true;
		this.scheduleSave();
	}

	/** vom Manuskript: ein getippter Absatz wurde übernommen; die Korrekturen ersetzen die bisherigen */
	commitEdits(/** @type {import('./model.js').Edit[]} */ edits) {
		this.editing = false;
		if (!this.canEdit) return;
		this.#remember(JSON.stringify(this.ann));
		this.#apply({ ...this.ann, edits });
	}

	/** Rückgängig: eine offene Eingabe wird verworfen, sonst gilt der vorige Stand wieder */
	undo() {
		if (this.editing) return this.textEdit?.discard();
		const prev = this.#undo.pop();
		this.canUndo = this.#undo.length > 0;
		if (prev) this.#apply(JSON.parse(prev));
	}

	#setPending(/** @type {boolean} */ on) {
		this.pending = on;
		this.store.recompute?.mark(on);
	}

	/**
	 * Neu berechnen (nur mit store.recompute): erst alles speichern, dann rechnet der Server das
	 * Transkript neu (Sprache neu erkennen, Sprecher zusammenlegen); Abspielstelle und Ausschnitt bleiben.
	 */
	async recompute() {
		const rc = this.store.recompute;
		if (!rc || this.computing) return;
		this.computing = true;
		try {
			if (!(await this.saveNow())) return this.status(m.te_recompute_unsaved(), 'fehler');
			const part = serverPart(this.ann);
			this.status(m.te_recompute_running(), 'offen');
			const r = await rc.run();
			if (!r.final) return this.status(m.te_recompute_failed({ reason: r.error || m.te_recompute_unknown() }), 'fehler');
			this.final = r.final;
			this.#computed = part;
			// was in der Zwischenzeit festgelegt wurde, bleibt vorgemerkt
			this.#setPending(serverPart(this.ann) !== part);
			this.rebuild();
			this.status(m.te_recomputed_at({ time: clock() }));
		} finally {
			this.computing = false;
		}
	}

	/**
	 * Name, Sprache und Zusammenlegen eines Sprechers. Ein Name gleich dem bisherigen wird nicht
	 * vermerkt; was leer bleibt, fällt weg, andere Angaben am Sprecher bleiben.
	 * @param {number} k
	 * @param {{ name: string, language: string, merge_into: string }} v
	 */
	setSpeaker(k, v) {
		const machine = this.ann.speakers[k]?.name ? '' : this.spkName(k);
		const name = v.name.trim() === machine ? '' : v.name.trim();
		const next = { name, language: v.language === 'auto' ? '' : v.language, merge_into: v.merge_into };
		this.change((ann) => {
			/** @type {Record<string, string>} */ const s = { ...ann.speakers[k] };
			for (const [key, x] of Object.entries(next))
				if (x) s[key] = x;
				else delete s[key];
			if (Object.keys(s).length) ann.speakers[k] = s;
			else delete ann.speakers[k];
		});
	}

	/** entfernt eine Korrektur aus der Liste (kind: 'edits' oder 'spans') */
	removeCorrection(/** @type {'edits' | 'spans'} */ kind, /** @type {number} */ i) {
		this.change((a) => void a[kind].splice(i, 1));
	}

	status(/** @type {string} */ text, kind = 'ok') {
		this.saveText = text;
		this.saveKind = kind;
	}

	scheduleSave() {
		clearTimeout(this.#saveTimer);
		this.status(m.te_saving(), 'offen');
		this.#saveTimer = setTimeout(() => this.saveNow(), 800);
	}

	/** speichert sofort, was noch nicht gespeichert ist; true, wenn danach nichts mehr aussteht */
	async saveNow() {
		clearTimeout(this.#saveTimer);
		this.textEdit?.commit();
		if (this.#saving) await this.#saving;
		if (!this.#dirty) return true;
		const ann = this.ann;
		this.#dirty = false;
		this.status(m.te_saving(), 'offen');
		this.#saving = this.store
			.save(ann, this.version)
			.then((r) => {
				if (r.status === 'saved' || r.status === 'unchanged') {
					if (r.version != null) this.version = r.version;
					// was der Speicher daraus gerechnet hat (zusammengelegt, Namen); getippt wurde inzwischen vielleicht weiter
					if (r.final) {
						this.final = r.final;
						this.rebuild();
					}
					if (r.languagesPending) this.languagesBusy = true;
					if (!this.#dirty && !this.editing)
						this.status(r.applyFailed ? m.te_saved_apply_later() : m.te_saved_at({ time: clock() }), r.applyFailed ? 'offen' : 'ok');
					return true;
				}
				if (r.status === 'conflict') {
					// jemand anderes hat inzwischen gespeichert: dessen Stand gilt, diese Änderung nicht
					clearTimeout(this.#saveTimer);
					this.#undo = [];
					this.canUndo = false;
					this.ann = r.annotations;
					this.version = r.version ?? this.version;
					if (r.final) this.final = r.final;
					this.rebuild();
					this.status(m.te_conflict(), 'fehler');
					return false;
				}
				this.#dirty = true;
				if (r.status === 'refused') {
					this.status(m.te_save_refused(), 'fehler');
					return false;
				}
				this.status(m.te_save_retry(), 'fehler');
				this.#saveTimer = setTimeout(() => this.saveNow(), 3000);
				return false;
			})
			.finally(() => (this.#saving = null));
		return this.#saving;
	}

	/** beim Verlassen: was noch aussteht, geht ohne Warten weg */
	flush() {
		this.textEdit?.commit();
		if (this.#dirty) this.saveNow();
	}
	get busy() {
		return !!this.#saving || this.#dirty;
	}

	/** Sprecher und Sprache für einen Bereich festlegen; speaker 'neu' nimmt die nächste freie Nummer */
	setSpan(/** @type {number} */ a, /** @type {number} */ b, /** @type {{ speaker?: string, language?: string }} */ what) {
		let speaker = what.speaker ?? '';
		if (speaker === 'neu') speaker = String(Math.max(-1, ...this.words.map((w) => w.speaker ?? -1), ...Object.keys(this.ann.speakers).map(Number)) + 1);
		const language = what.language ?? '';
		if (!speaker && !language) return;
		this.change((ann) => {
			/** @type {import('./model.js').Span} */ const s = { start: +a.toFixed(2), end: +b.toFixed(2) };
			if (language) s.language = language;
			if (speaker !== '') s.speaker = speaker;
			ann.spans.push(s);
		});
	}

	/** Text in eine Pause einfügen (Bereich ohne Wörter) */
	insertText(/** @type {number} */ a, /** @type {number} */ b, /** @type {string} */ text) {
		text = text.trim();
		if (!text) return;
		this.change((ann) => void ann.edits.push({ start: r3(a), end: r3(Math.max(b, a + 0.3)), text, mode: 'insert' }));
	}

	/** Zeitanker versetzen: in der Zeitleiste auf eine andere Zeit (drop fehlt) oder im Text auf ein anderes Wort */
	moveAnchorTo(/** @type {number} */ from, /** @type {number} */ to, /** @type {number | undefined} */ drop) {
		this.change((ann) => void ann.edits.push({ start: r3(from), end: r3(to), mode: 'anchor', ...(drop == null ? {} : { drop: r3(drop) }) }));
	}

	async loadThumbs() {
		try {
			const t = await this.store.thumbs();
			if (!t) return;
			const img = new Image();
			img.src = t.url;
			await img.decode();
			this.thumbs = { meta: t.meta, img, url: t.url };
		} catch {
			// ohne Vorschaubilder geht es auch
		}
	}

	// ---------------- Sprecherfilter ----------------

	isOpened(/** @type {number} */ t) {
		return this.opened.some((r) => t >= r.a - 0.3 && t <= r.b + 0.3);
	}
	setHidden(/** @type {Set<number>} */ set) {
		if (this.speakers.length && this.speakers.every((k) => set.has(k))) return; // einer bleibt sichtbar
		this.hidden = set;
		this.opened = [];
		this.wordbar = null;
	}
	toggleSpeaker(/** @type {number} */ k) {
		const s = new Set(this.hidden);
		if (s.has(k)) s.delete(k);
		else s.add(k);
		this.setHidden(s);
	}
	focusSpeaker(/** @type {number} */ k) {
		this.setHidden(this.focused === k ? new Set() : new Set(this.speakers.filter((x) => x !== k)));
	}
	showAll() {
		this.setHidden(new Set());
	}
	/** klappt eine eingeklappte Stelle auf oder wieder zu */
	toggleFold(/** @type {import('./model.js').Klappe} */ g) {
		if (g.open) this.opened = this.opened.filter((r) => r.b < g.start - 0.3 || r.a > g.end + 0.3);
		else this.opened = [...this.opened, { a: g.start, b: g.end }];
	}
	filterLabel = $derived.by(() => {
		if (!this.hidden.size) return '';
		const off = this.speakers.filter((k) => this.hidden.has(k));
		if (this.focused != null) return m.te_filter_only({ name: this.spkName(this.focused) });
		return off.length === 1 ? m.te_filter_hidden_one({ name: this.spkName(off[0]) }) : m.te_filter_hidden_many({ count: off.length });
	});

	// ---------------- Abspielen ----------------

	attach(/** @type {HTMLVideoElement} */ media) {
		this.media = media;
		const on = (/** @type {string} */ ev, /** @type {() => void} */ fn) => media.addEventListener(ev, fn);
		on('play', () => {
			this.playing = true;
			cancelAnimationFrame(this.#raf);
			this.#loop();
		});
		on('pause', () => {
			this.playing = false;
			cancelAnimationFrame(this.#raf);
			this.time = media.currentTime;
		});
		on('seeked', () => (this.time = media.currentTime));
		on('timeupdate', () => {
			if (media.paused) this.time = media.currentTime;
		});
		on('loadedmetadata', () => (this.mediaDuration = media.duration || 0));
		if (this.mediaUrl) {
			media.src = this.mediaUrl;
			media.playbackRate = this.rate;
		}
		return () => {
			cancelAnimationFrame(this.#raf);
			media.pause();
			this.media = null;
		};
	}

	#loop = () => {
		const m = this.media;
		if (!m) return;
		if (this.loop && (m.currentTime >= this.loop.b || m.currentTime < this.loop.a - 0.5)) m.currentTime = this.loop.a;
		else if (this.stopAt != null) {
			if (m.currentTime >= this.stopAt) {
				m.pause();
				this.stopAt = null;
			}
		} else if (this.playList) this.#nextClip();
		else if (this.playable) this.#skipHidden();
		this.time = m.currentTime;
		const v = this.view,
			t = this.time,
			w = v.b - v.a;
		if (this.follow && (t > v.b - w * 0.05 || t < v.a)) this.setView(t - w * 0.1, t - w * 0.1 + w);
		this.#raf = requestAnimationFrame(this.#loop);
	};

	// am Ende eines Clips geht es mit dem nächsten weiter, nach dem letzten hält das Abspielen an
	#nextClip() {
		const m = /** @type {HTMLVideoElement} */ (this.media),
			pl = /** @type {{ a: number, b: number, id: string }[]} */ (this.playList);
		if (m.currentTime < pl[this.playIdx].b) return;
		if (this.playIdx + 1 < pl.length) m.currentTime = pl[++this.playIdx].a;
		else {
			m.pause();
			this.playList = null;
		}
	}

	// mit Sprecherfilter springt das Abspielen über Ausgeblendetes und hält nach der letzten sichtbaren Stelle
	#skipHidden() {
		const m = /** @type {HTMLVideoElement} */ (this.media),
			t = m.currentTime,
			pl = /** @type {{a: number, b: number}[]} */ (this.playable);
		if (pl.some((r) => r.a <= t && t < r.b)) return;
		const next = pl.find((r) => r.a > t);
		if (next) m.currentTime = next.a;
		else m.pause();
	}

	/** spielt ab der Stelle t (Klick auf Zeitmarke, Zeitleiste, Strg+Klick ins Wort) */
	seek(/** @type {number} */ t, play = true) {
		const m = this.media;
		if (!m) return;
		m.currentTime = Math.max(0, t - 0.05);
		this.time = m.currentTime;
		this.userScroll = 0;
		this.caretT = null;
		this.stopAt = null;
		this.loop = null;
		this.playList = null;
		if (play && m.paused) m.play().catch(() => {});
	}

	/** spielt einen markierten Bereich und hält an seinem Ende; mit loop immer wieder von vorn */
	playRange(/** @type {number} */ a, /** @type {number} */ b, loop = false) {
		const m = this.media;
		if (!m) return;
		m.currentTime = a;
		this.time = a;
		this.userScroll = 0;
		this.caretT = null;
		this.stopAt = loop ? null : b;
		this.loop = loop ? { a, b } : null;
		this.playList = null;
		m.play().catch(() => {});
	}

	/** Abspielknopf: hält an, sonst spielt er ab der angeklickten Stelle oder dort, wo er stand */
	toggle() {
		const m = this.media;
		if (!m) return;
		if (!m.paused) return m.pause();
		if (this.caretT != null) {
			m.currentTime = Math.max(0, this.caretT - 0.05);
			this.caretT = null;
			this.stopAt = null;
			this.loop = null;
			this.playList = null;
		} else if (this.playable?.length && !this.playable.some((r) => r.b > m.currentTime)) m.currentTime = this.playable[0].a;
		this.userScroll = 0;
		m.play().catch(() => {});
	}

	skip(/** @type {number} */ dt) {
		const m = this.media;
		if (!m) return;
		m.currentTime = Math.max(0, Math.min(this.duration(), m.currentTime + dt));
		this.time = m.currentTime;
		this.userScroll = 0;
	}

	setRate(/** @type {number} */ r) {
		this.rate = r;
		if (this.media) this.media.playbackRate = r;
	}

	// ---------------- Clips ----------------
	// Stellen mit Titel und Schlagworten; sie werden mit den Korrekturen gespeichert und ändern nichts am Text

	/**
	 * legt einen Clip an (ohne id) oder ändert ihn; ohne Titel nimmt er die ersten Wörter
	 * @param {{ id?: string }} old
	 * @param {{ start: number, end: number, title: string, tags: string[] }} v
	 */
	saveClip(old, v) {
		const clip = {
			id: old.id || 'c' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
			start: r3(v.start),
			end: r3(v.end),
			title: v.title.trim() || defaultTitle(rangeText(this.words, v.start, v.end)),
			tags: v.tags
		};
		this.change((ann) => {
			ann.clips = old.id ? ann.clips.map((c) => (c.id === old.id ? clip : c)) : [...ann.clips, clip];
			ann.clips.sort((a, b) => a.start - b.start);
		});
		this.activeClip = clip.id;
		this.sel = null;
		return clip;
	}

	deleteClip(/** @type {string} */ id) {
		this.change((ann) => void (ann.clips = ann.clips.filter((c) => c.id !== id)));
		if (this.clipSel.has(id)) this.toggleClipSel(id, false);
		if (this.playList?.some((r) => r.id === id)) {
			this.media?.pause();
			this.playList = null;
		}
		this.sel = null;
	}

	toggleClipSel(/** @type {string} */ id, /** @type {boolean} */ on) {
		const s = new Set(this.clipSel);
		if (on) s.add(id);
		else s.delete(id);
		this.clipSel = s;
	}

	/** zeigt einen Clip in Zeitleiste und Text; die Abspielstelle steht an seinem Anfang, mit play läuft nur er */
	showClip(/** @type {import('./model.js').Clip} */ c, play = false) {
		this.activeClip = c.id;
		this.sel = { a: c.start, b: c.end };
		const v = this.view;
		if (c.start < v.a || c.end > v.b) {
			const pad = Math.max(1, (c.end - c.start) * 0.15),
				w = Math.max(v.b - v.a, c.end - c.start + 2 * pad);
			this.setView(c.start - pad, c.start - pad + w);
		}
		this.revealT = { t: c.start, n: (this.revealT?.n ?? 0) + 1 };
		if (play) return this.playClips([c]);
		const m = this.media;
		if (!m) return;
		m.currentTime = c.start;
		this.time = c.start;
		this.caretT = this.stopAt = this.loop = this.playList = null;
	}

	/** spielt Clips in zeitlicher Folge; was dazwischen liegt, wird übersprungen */
	playClips(/** @type {import('./model.js').Clip[]} */ list) {
		const m = this.media;
		if (!m || !list.length) return;
		this.playList = [...list].sort((x, y) => x.start - y.start).map((c) => ({ a: c.start, b: c.end, id: c.id }));
		this.playIdx = 0;
		this.loop = null;
		this.stopAt = null;
		this.caretT = null;
		this.userScroll = 0;
		m.currentTime = this.playList[0].a;
		this.time = m.currentTime;
		m.play().catch(() => {});
	}

	/** lädt Clips herunter: einen oder einen Zusammenschnitt, als mp3, mp4 oder txt; vorher wird gespeichert */
	async exportClips(/** @type {import('./model.js').Clip[]} */ list, /** @type {string} */ format) {
		if (!list.length) return;
		if (!(await this.saveNow())) return this.status(m.te_clips_not_saved(), 'fehler');
		this.status(format === 'txt' ? m.te_clip_text_building() : list.length > 1 ? m.te_clips_cutting() : m.te_clip_cutting(), 'offen');
		const error = await this.store.exportClips(list, format, (text) => this.status(text, 'offen'));
		if (error) this.status(m.te_export_failed({ reason: error }), 'fehler');
		else this.status(m.te_exported());
	}

	/** lädt das Transkript herunter; vorher wird gespeichert, damit gerade Getipptes darin steht */
	async exportTranscript(/** @type {string} */ format) {
		if (!(await this.saveNow())) return this.status(m.te_export_not_saved(), 'fehler');
		const error = await this.store.exportTranscript(format);
		if (error) this.status(m.te_export_failed({ reason: error }), 'fehler');
	}

	// ---------------- Zeitleiste ----------------

	setView(/** @type {number} */ a, /** @type {number} */ b) {
		const d = this.duration(),
			w = Math.max(2, Math.min(d, b - a));
		a = Math.max(0, Math.min(a, d - w));
		this.view = { a, b: a + w };
	}
	zoom(/** @type {number} */ f) {
		const m = (this.view.a + this.view.b) / 2,
			w = (this.view.b - this.view.a) * f;
		this.setView(m - w / 2, m + w / 2);
	}
}
