<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Bibliothek, die Startseite. Links die Bereiche: alle Aufnahmen, hochgeladene, Live-Aufnahmen
     und die verbundenen Ordner, deren Aufnahmen bleiben, wo sie sind (ihr Transkript liegt als
     „….transcribe.json“ daneben, internal/sidecar). Rechts für den Bereich: was gerade transkribiert
     wird oder gescheitert ist, die Aufnahmen ohne Transkript zum Einreihen und die transkribierten
     als Liste, jede mit dem Anfang ihres Transkripts, wer spricht und wie viel schon von Hand
     korrigiert ist; ein Klick öffnet sie im Editor. Jede Aufnahme lässt sich benennen und
     verschlagworten (Beschriften.svelte); die Schlagworte stehen links als Bereiche. Ausgeblendete
     verschwinden aus allen Bereichen und stehen nur unter „Ausgeblendet“; so lässt sich auch in einem
     verbundenen Ordner aufräumen, in dem Transcribe nichts löscht. Jede Aufnahme, mit Transkript
     oder ohne, lässt sich vorn in ihrer Zeile anhören; dann klebt unten eine Abspielleiste. Das
     i-Symbol öffnet ihre Details (Dateiinfo.svelte). Steht unter / und, für alte Lesezeichen, unter
     /queue. -->
<script>
	import { onMount, untrack } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import AuftragsOptionen from '$lib/AuftragsOptionen.svelte';
	import Beschriften from '$lib/Beschriften.svelte';
	import Dateiinfo from '$lib/Dateiinfo.svelte';
	import Abspielen from '$lib/live/Abspielen.svelte';
	import { auftraege, geaendert } from '$lib/auftraege.svelte.js';
	import { beschriften, dauer, einreihen, einzeln, getJSON, groesse, post, upload, verbindung, vorgaben } from '$lib/api.js';
	import { sprachname } from '$lib/live/stand.svelte.js';
	import MoliriLogo from '$lib/MoliriLogo.svelte';
	import { HANDBUCH, MIT_MOLIRI, MOLIRI } from '$lib/moliri-links.js';

	/** @typedef {{ id: number, name: string }} Sprecher */
	/** @typedef {{ duration: number, speakers: Sprecher[], preview: string, edits: number, clips: number, done: number }} Inhalt */
	/** was ffprobe über die Datei sagt (der Server fragt es im Hintergrund und merkt es sich): Länge in Sekunden, Format und Ton in
	 * Worten, wann aufgenommen (RFC 3339, wie die Datei es nennt)
	 * @typedef {{ duration: number, format: string, codec: string, recorded: string }} Medium */
	/** title: der Name in Transcribe (sonst der der Datei), file: der Name der Datei ohne Endung, size: Bytes, added: zuletzt
	 * geändert, created: angelegt (Sekunden)
	 * @typedef {{ name: string, stem: string, title: string, file: string, tags: string[], hidden: boolean, video: boolean, added: number, created?: number,
	 *   size: number, media?: Medium, summary?: Inhalt, source: 'upload' | 'live' | 'folder', folder?: string, dir?: string, path?: string, sidecar?: boolean }} Aufnahme */
	/** @typedef {{ id: string, audio: string, stem: string, status: string, progress: number, step: string | null,
	 *   log: string[] | null, started: number | null, finished: number | null, error: string | null, note: string | null,
	 *   options: { preset: string, speakers: number | null, language?: string, engine?: string } }} Auftrag */
	/** @typedef {{ path: string, name: string, exists: boolean }} Ordner */
	/** @typedef {{ folders: Ordner[], sidecar: boolean, pick: boolean, hint?: { path: string, name: string, count: number } }} Ordnerstand */

	const STATUS = /** @type {Record<string, string>} */ ({
		queued: 'wartet',
		running: 'läuft',
		paused: 'pausiert',
		canceling: 'bricht ab',
		error: 'Fehler'
	});
	const LAUFEND = ['queued', 'running', 'paused', 'canceling'];

	const SORTEN = /** @type {const} */ ([
		['zuletzt', 'Zuletzt bearbeitet'],
		['name', 'Name'],
		['laenge', 'Länge']
	]);

	/** die Bereiche ohne die Ordner; ein Ordner ist sein Pfad */
	const BEREICHE = /** @type {const} */ ([
		['alle', 'Alle Aufnahmen'],
		['upload', 'Hochgeladen'],
		['live', 'Live-Aufnahmen']
	]);

	// Was die Bibliothek zuletzt zeigte, steht beim Öffnen sofort da; was der Server dann sagt, ersetzt es.
	/** @type {Aufnahme[] | null} */
	let aufnahmen = $state(vorrat('aufnahmen'));
	/** @type {Auftrag[] | null} */
	let jobs = $state(vorrat('auftraege'));
	/** @type {Ordnerstand} */
	let ordner = $state(vorrat('ordner') ?? { folders: [], sidecar: true, pick: false });
	let offline = $state(false);
	/** die Transkriptionen ruhen: von Hand pausiert (bis „Fortsetzen“), oder solange live transkribiert wird */
	let angehalten = $state(false);
	let liveHaelt = $state(false);
	let schaltet = $state(false);
	/** Namen der angekreuzten Aufnahmen */
	let gewaehlt = $state(/** @type {string[]} */ ([]));
	/** @type {import('$lib/api.js').Optionen} */
	let optionen = $state({ speakers: '', language: 'auto' });
	/** wo erkannt wird (Einstellungen): local, api oder both */
	let erkennung = $state('local');
	let meldung = $state('');
	let laedtHoch = $state(false);
	let reiht = $state(false);
	let suche = $state('');
	let sorte = $state(/** @type {'zuletzt' | 'name' | 'laenge'} */ ('zuletzt'));
	/** der gewählte Bereich: alle, upload, live oder der Pfad eines Ordners */
	let bereich = $state(gemerkt());
	/** „Ordner verbinden“ ist offen, als Dialog über dem Bereich, der gewählt bleibt */
	let verbinden = $state(false);
	/** @type {HTMLDialogElement} */
	let verbindenDialog;
	/** was beim Verbinden schiefging, im Dialog gezeigt */
	let verbindenFehler = $state('');
	let pfad = $state('');
	let waehlt = $state(false);
	/** Uhr für die Laufzeit laufender Aufträge, in Sekunden */
	let jetzt = $state(Date.now() / 1000);
	/** die Aufnahme, nach deren Löschen gerade gefragt wird */
	let loeschen = $state(/** @type {Aufnahme | null} */ (null));
	let loescht = $state(false);
	/** @type {HTMLDialogElement} */
	let rueckfrage;
	/** @type {Beschriften} */
	let beschrifter;
	/** „Rückgängig“ neben der Meldung, solange sie steht */
	let zurueck = $state(/** @type {{ meldung: string, tun: () => void } | null} */ (null));
	/** die Aufnahme in der Abspielleiste, und ob sie gerade steht */
	let spielt = $state(/** @type {Aufnahme | null} */ (null));
	let pausiert = $state(true);
	/** @type {Abspielen | undefined} */
	let spieler = $state();
	/** @type {Dateiinfo} */
	let details;

	/** was die Bibliothek beim letzten Mal unter k zeigte
	 * @param {string} k */
	function vorrat(k) {
		try {
			return JSON.parse(localStorage.getItem('bib:vorrat:' + k) ?? 'null');
		} catch {
			return null;
		}
	}

	/** @type {Record<string, string>} */
	const gemerkteTexte = {};
	/** @param {string} k @param {unknown} v */
	function merke(k, v) {
		try {
			const t = JSON.stringify(v);
			if (gemerkteTexte[k] === t) return;
			gemerkteTexte[k] = t;
			localStorage.setItem('bib:vorrat:' + k, t);
		} catch {
			// voll oder ohne Speicher: dann wartet die Bibliothek beim nächsten Öffnen auf den Server
		}
	}

	function gemerkt() {
		try {
			return localStorage.getItem('bib:bereich') || 'alle';
		} catch {
			return 'alle';
		}
	}

	/** @param {string} b */
	function zeige(b) {
		bereich = b;
		schliesseVerbinden();
		gewaehlt = [];
		suche = '';
		try {
			localStorage.setItem('bib:bereich', b);
		} catch {
			// ohne Speicher beginnt die Bibliothek wieder bei allen Aufnahmen
		}
	}

	/** Gehört die Aufnahme in den Bereich? Ausgeblendete nur unter „Ausgeblendet“; ein Schlagwort ist „tag:…“.
	 * @param {Aufnahme} a @param {string} b */
	const gehoert = (a, b) =>
		b === 'versteckt'
			? a.hidden
			: !a.hidden &&
				(b === 'alle' ||
					a.source === b ||
					(a.source === 'folder' && a.folder === b) ||
					(b.startsWith('tag:') && a.tags.includes(b.slice(4))));

	const nachName = $derived(new Map((aufnahmen ?? []).map((a) => [a.name, a])));
	/** je Aufnahme ihr jüngster Auftrag (die Warteschlange führt sie in der Reihenfolge des Einreihens) */
	const letzter = $derived(new Map((jobs ?? []).map((j) => [j.audio, j])));
	/** die Schlagworte der sichtbaren Aufnahmen, nach dem Alphabet */
	const schlagworte = $derived(
		[...new Set((aufnahmen ?? []).filter((a) => !a.hidden).flatMap((a) => a.tags))].sort((x, y) => x.localeCompare(y, 'de'))
	);
	/** alle vergebenen, auch an ausgeblendeten, zum Antippen beim Beschriften */
	const alleSchlagworte = $derived([...new Set((aufnahmen ?? []).flatMap((a) => a.tags))]);
	/** der gewählte Ordner, wenn einer gewählt ist */
	const imOrdner = $derived(ordner.folders.find((o) => o.path === bereich) ?? null);
	/** was gerade transkribiert wird oder zuletzt gescheitert ist, im Bereich */
	const inArbeit = $derived(
		[...letzter.values()].filter((j) => {
			if (!LAUFEND.includes(j.status) && j.status !== 'error') return false;
			const a = nachName.get(j.audio);
			return a ? gehoert(a, bereich) : bereich === 'alle';
		})
	);
	const laufend = $derived(inArbeit.filter((j) => LAUFEND.includes(j.status)));
	const imBereich = $derived((aufnahmen ?? []).filter((a) => gehoert(a, bereich)));
	/** Aufnahmen ohne Transkript, die auch nicht gerade in Arbeit sind */
	const neu = $derived(imBereich.filter((a) => !a.summary && !inArbeit.some((j) => j.audio === a.name)));
	const fertig = $derived(imBereich.filter((a) => a.summary));

	/** wie viele Aufnahmen ein Bereich hat, und wie viele davon noch kein Transkript */
	const zahlen = $derived.by(() => {
		/** @type {Record<string, { alle: number, offen: number }>} */
		const z = {};
		for (const b of [...BEREICHE.map(([b]) => b), ...ordner.folders.map((o) => o.path), ...schlagworte.map((t) => 'tag:' + t), 'versteckt']) {
			const in_ = (aufnahmen ?? []).filter((a) => gehoert(a, b));
			z[b] = { alle: in_.length, offen: in_.filter((a) => !a.summary).length };
		}
		return z;
	});

	const regal = $derived.by(() => {
		const q = suche.trim().toLowerCase();
		const treffer = q
			? fertig.filter((a) =>
					[a.title, a.file, a.dir ?? '', a.summary?.preview ?? '', ...a.tags, ...(a.summary?.speakers ?? []).map((s) => s.name)].some((t) =>
						t.toLowerCase().includes(q)
					)
				)
			: fertig;
		const nach = {
			zuletzt: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => (b.summary?.done ?? 0) - (a.summary?.done ?? 0),
			name: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => a.title.localeCompare(b.title, 'de'),
			laenge: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => (b.summary?.duration ?? 0) - (a.summary?.duration ?? 0)
		}[sorte];
		return treffer.toSorted(nach);
	});

	/** wie ein Auftrag seine Aufnahme nennt: wie die Bibliothek, sonst der Name ohne Endung
	 * @param {string} name */
	const titel = (name) => nachName.get(name)?.title ?? name.replace(/^~[0-9a-f]{10}~/, '').replace(/\.[^.]+$/, '');

	/** wo eine Aufnahme liegt, wenn das neben ihrem Namen etwas sagt: ihr Unterordner und, unter
	 * allen Aufnahmen, ihr Ordner
	 * @param {Aufnahme} a */
	function ort(a) {
		if (a.source !== 'folder') return bereich === 'alle' && a.source === 'live' ? 'Live' : '';
		const name = ordner.folders.find((o) => o.path === a.folder)?.name ?? '';
		return [bereich === 'alle' ? name : '', a.dir ?? ''].filter(Boolean).join(' / ');
	}

	/** wann, so kurz wie möglich: heute um 14:05, gestern, 12. Okt. oder 12. Okt. 2025
	 * @param {number} sek */
	function wann(sek) {
		const d = new Date(sek * 1000);
		const h = new Date();
		const tage = Math.round(
			(new Date(h.getFullYear(), h.getMonth(), h.getDate()).getTime() -
				new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()) /
				864e5
		);
		if (tage === 0) return `heute, ${d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}`;
		if (tage === 1) return 'gestern';
		return d.toLocaleDateString('de-DE', {
			day: 'numeric',
			month: 'short',
			...(d.getFullYear() !== h.getFullYear() && { year: 'numeric' })
		});
	}

	/** wann die Aufnahme entstand: wie die Datei selbst es sagt, sonst das Frühere von angelegt und geändert (eine kopierte
	 * Datei ist neu angelegt, aber nicht neu geändert)
	 * @param {Aufnahme} a @returns {[number, string]} Sekunden und was das ist */
	function entstanden(a) {
		const t = a.media?.recorded ? Date.parse(a.media.recorded) / 1000 : NaN;
		if (!isNaN(t)) return [t, 'Aufgenommen'];
		return a.created && a.created < a.added ? [a.created, 'Angelegt'] : [a.added, 'Geändert'];
	}

	/** @param {number} sek */
	const datum = (sek) =>
		new Date(sek * 1000).toLocaleString('de-DE', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' });

	/** das Format, wie die Endung es nennt: MP3, WAV, MP4
	 * @param {string} name */
	const endung = (name) => (name.match(/\.([^.]+)$/)?.[1] ?? '').toUpperCase();

	/** Format und Ton in Worten, für den Hinweis über dem Format
	 * @param {Aufnahme} a */
	const formatInWorten = (a) => [a.media?.format, a.media?.codec].filter(Boolean).join(' · ') || undefined;

	/** die Optionen eines Auftrags in Worten
	 * @param {Auftrag} j */
	function wie(j) {
		const o = j.options;
		const sprecher = o.speakers || ({ interview: 2, vortrag: 1 })[o.preset];
		return [
			sprecher ? `${sprecher} Sprecher` : 'Sprecher automatisch',
			o.language && o.language !== 'auto' ? sprachname(o.language) : '',
			o.engine === 'api' ? 'über die API' : ''
		]
			.filter(Boolean)
			.join(' · ');
	}

	/** gerade fertig geworden: in den letzten 24 Stunden von einem Auftrag transkribiert
	 * @param {Aufnahme} a */
	function frisch(a) {
		const j = letzter.get(a.name);
		return j?.status === 'done' && !!j.finished && jetzt - j.finished < 86400;
	}

	/** @type {ReturnType<typeof setTimeout> | undefined} */
	let nachProbe;

	/** zählt die Abfragen der Liste: eine ältere, die erst nach einer neueren ankommt, gilt nicht mehr */
	let abfrage = 0;

	async function ladeAufnahmen() {
		const diese = ++abfrage;
		let probt = 0;
		try {
			const r = await fetch('/api/index');
			if (!r.ok) throw new Error(r.statusText);
			probt = Number(r.headers.get('X-Probing')) || 0;
			/** @type {(Aufnahme & Record<string, unknown>)[]} */
			const liste = await r.json();
			if (diese !== abfrage) return;
			aufnahmen = liste.map(({ engines: _e, speakers: _s, events: _v, url: _u, ...a }) => /** @type {Aufnahme} */ (a));
		} catch {
			aufnahmen ??= [];
			return;
		}
		merke('aufnahmen', aufnahmen);
		// der Server liest Länge und Format neuer Dateien noch: gleich noch einmal fragen
		clearTimeout(nachProbe);
		if (probt) nachProbe = setTimeout(ladeAufnahmen, 1500);
		// ein Schlagwort, das keine Aufnahme mehr trägt, und „Ausgeblendet“ ohne Aufnahme sind keine Bereiche mehr
		if ((bereich.startsWith('tag:') || bereich === 'versteckt') && !zahlen[bereich]?.alle) zeige('alle');
	}

	async function ladeOrdner() {
		try {
			ordner = await getJSON('/api/folders');
			merke('ordner', ordner);
		} catch {
			// bleibt, wie es war
		}
		// ein Ordner, der nicht mehr verbunden ist, ist kein Bereich mehr
		if (!['alle', 'upload', 'live'].includes(bereich) && !ordner.folders.some((o) => o.path === bereich)) zeige('alle');
	}

	const server = verbindung();
	let fragt = false;

	async function ladeAuftraege() {
		// ein langsamer Server bekommt keine zweite Frage, solange die erste offen ist
		if (fragt) return;
		fragt = true;
		const vorher = laufend.length;
		try {
			const q = await getJSON('/api/queue', { signal: AbortSignal.timeout(8000) });
			jobs = q.jobs;
			merke('auftraege', q.jobs.map((/** @type {Auftrag} */ j) => ({ ...j, log: null })));
			angehalten = !!q.paused;
			liveHaelt = !!q.live;
			server.ok();
			offline = false;
		} catch (e) {
			offline = server.fehler(e);
			return;
		} finally {
			fragt = false;
		}
		// ein Auftrag ist fertig geworden: sein Transkript steht jetzt in der Liste
		if (laufend.length < vorher) ladeAufnahmen();
	}

	/** Einen Ordner verbinden (action add) oder trennen (remove), den Vorschlag ablehnen (decline).
	 * @param {'add' | 'remove' | 'decline'} action @param {string} path */
	async function ordnerAendern(action, path) {
		try {
			ordner = await post('/api/folders', { action, path });
		} catch (e) {
			meldeVerbinden(/** @type {Error} */ (e).message);
			return false;
		}
		await ladeAufnahmen();
		return true;
	}

	function oeffneVerbinden() {
		verbindenFehler = '';
		verbinden = true;
		verbindenDialog.showModal();
	}

	function schliesseVerbinden() {
		if (verbindenDialog?.open) verbindenDialog.close();
		verbinden = false;
	}

	/** Steht der Dialog offen, gehört ein Fehler dorthin, sonst in die Meldung.
	 * @param {string} text */
	function meldeVerbinden(text) {
		if (verbinden) verbindenFehler = text;
		else meldung = text;
	}

	/** @param {string} path */
	async function verbinde(path) {
		verbindenFehler = '';
		if (!path.trim()) {
			meldeVerbinden('Bitte einen Ordner wählen oder seinen Pfad einfügen.');
			return;
		}
		if (!(await ordnerAendern('add', path))) return;
		const neuer = ordner.folders.at(-1);
		pfad = '';
		if (neuer) {
			zeige(neuer.path);
			const n = zahlen[neuer.path]?.alle ?? 0;
			meldung = `„${neuer.name}“ ist verbunden: ${n === 1 ? 'eine Aufnahme' : `${n} Aufnahmen`}.`;
		}
	}

	/** Den Ordner im Dialog von Windows wählen und gleich verbinden. */
	async function waehlen() {
		waehlt = true;
		try {
			const { path } = await post('/api/folders/pick');
			if (path) await verbinde(path);
		} catch (e) {
			meldeVerbinden(/** @type {Error} */ (e).message);
		} finally {
			waehlt = false;
		}
	}

	/** @param {Ordner} o */
	async function trennen(o) {
		if (await ordnerAendern('remove', o.path)) {
			zeige('alle');
			meldung = `„${o.name}“ ist getrennt. Die Aufnahmen und Transkripte darin bleiben, wo sie sind.`;
		}
	}

	async function transkribieren() {
		if (!gewaehlt.length) {
			meldung = 'Bitte mindestens eine Aufnahme wählen.';
			return;
		}
		reiht = true;
		try {
			const n = await einreihen(gewaehlt, optionen);
			meldung = n ? (n === 1 ? 'Eine Aufnahme wird transkribiert.' : `${n} Aufnahmen werden transkribiert.`) : 'Schon in Arbeit.';
			gewaehlt = [];
			geaendert();
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		} finally {
			reiht = false;
		}
	}

	/** Ein Transkript neu berechnen: Korrekturen von Hand bleiben dabei erhalten.
	 * @param {Aufnahme} a */
	async function neuTranskribieren(a) {
		try {
			const n = await einreihen([a.name], optionen);
			meldung = n ? `„${a.title}“ wird neu transkribiert. Deine Korrekturen bleiben erhalten.` : 'Schon in Arbeit.';
			geaendert();
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		}
	}

	/** @param {Auftrag} j @param {'cancel' | 'retry' | 'remove'} was */
	async function aktion(j, was) {
		try {
			await post(`/api/queue/${j.id}/${was}`);
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		}
		geaendert();
	}

	/** Von Hand pausieren oder fortsetzen; die laufende Transkription bleibt dabei an ihrer Stelle. */
	async function pause() {
		schaltet = true;
		try {
			await post(angehalten ? '/api/queue/resume' : '/api/queue/pause');
			angehalten = !angehalten;
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		} finally {
			schaltet = false;
		}
		geaendert();
	}

	async function alleAbbrechen() {
		try {
			await post('/api/queue/cancel-all');
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		}
		geaendert();
	}

	/** @param {Event & { currentTarget: HTMLInputElement }} e */
	async function hochladen(e) {
		const input = e.currentTarget;
		laedtHoch = true;
		try {
			const namen = [];
			for (const f of input.files ?? []) {
				meldung = `Lade ${f.name} hoch …`;
				try {
					namen.push(await upload(f));
				} catch (err) {
					meldung = `${f.name}: ${/** @type {Error} */ (err).message}`;
					return;
				}
			}
			input.value = '';
			meldung = namen.length === 1 ? 'Hochgeladen. Sie steht jetzt unter „Noch nicht transkribiert“.' : 'Hochgeladen.';
			// Hochgeladenes steht unter „Hochgeladen“ und ist gleich angekreuzt, ein Klick auf „Transkribieren“ genügt
			if (bereich !== 'alle' && bereich !== 'upload') zeige('upload');
			gewaehlt = [...new Set([...gewaehlt, ...namen])];
			ladeAufnahmen();
		} finally {
			laedtHoch = false;
		}
	}

	/** Wird die Aufnahme gerade transkribiert? Dann lässt sie sich nicht löschen; wartet sie nur, wird der Auftrag mit ihr abgebrochen.
	 * @param {Aufnahme} a */
	function inBearbeitung(a) {
		const s = letzter.get(a.name)?.status;
		return s === 'running' || s === 'canceling';
	}

	/** Löschen lässt sich, was in Transcribe liegt; in einem verbundenen Ordner löscht es nie etwas.
	 * @param {Aufnahme} a */
	function fragLoeschen(a) {
		loeschen = a;
		rueckfrage.showModal();
	}

	async function loeschenBestaetigt() {
		const a = loeschen;
		if (!a) return;
		loescht = true;
		try {
			await post(`/api/delete?name=${encodeURIComponent(a.name)}`);
			meldung = `„${a.title}“ ist gelöscht.`;
			gewaehlt = gewaehlt.filter((n) => n !== a.name);
			if (spielt?.name === a.name) spielt = null;
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		} finally {
			loescht = false;
			rueckfrage.close();
		}
		ladeAufnahmen();
		geaendert();
	}

	/** @param {Aufnahme} a */
	function fragBeschriften(a) {
		beschrifter.open(a);
	}

	/** Die Aufnahme anhören, oder anhalten und weiterspielen, wenn sie schon in der Leiste ist.
	 * @param {Aufnahme} a */
	function anhoeren(a) {
		if (spielt?.name === a.name && spieler) spieler.umschalten();
		else spielt = a;
	}

	/** Die Details spielen selbst ab; die Leiste geht dafür zu.
	 * @param {Aufnahme} a */
	function zeigeDetails(a) {
		spielt = null;
		details.open(a);
	}

	/** Gespeichert im Dialog: gleich zeigen, dann vom Server holen.
	 * @param {{ stem: string }} a @param {{ title: string, tags: string[], hidden: boolean }} stand */
	function beschriftet(a, stand) {
		const warVersteckt = aufnahmen?.find((x) => x.stem === a.stem)?.hidden;
		for (const x of aufnahmen ?? []) if (x.stem === a.stem) Object.assign(x, stand);
		if (warVersteckt !== undefined && stand.hidden !== warVersteckt) sichtbarGemacht(a.stem, stand.title, stand.hidden);
		else meldung = `„${stand.title}“ ist gespeichert.`;
		ladeAufnahmen();
	}

	/** Ausblenden oder wieder einblenden, ohne Dialog, mit „Rückgängig“.
	 * @param {Aufnahme} a @param {boolean} hidden */
	async function ausblenden(a, hidden) {
		try {
			const stand = await beschriften(a.stem, { hidden });
			for (const x of aufnahmen ?? []) if (x.stem === a.stem) Object.assign(x, stand);
			sichtbarGemacht(a.stem, stand.title, hidden);
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		}
		gewaehlt = gewaehlt.filter((n) => n !== a.name);
		ladeAufnahmen();
	}

	/** @param {string} stem @param {string} title @param {boolean} hidden */
	function sichtbarGemacht(stem, title, hidden) {
		meldung = hidden
			? `„${title}“ ist ausgeblendet. Die Datei bleibt, wo sie ist; links unter „Ausgeblendet“ lässt sie sich wieder einblenden.`
			: `„${title}“ ist wieder eingeblendet.`;
		const a = aufnahmen?.find((x) => x.stem === stem);
		zurueck = a ? { meldung, tun: () => ausblenden(a, !hidden) } : null;
	}

	/** was von Hand am Transkript getan wurde: „2 Korrekturen von Hand und 1 Clip“
	 * @param {Inhalt} s */
	function arbeitInWorten(s) {
		return [
			s.edits === 1 ? '1 Korrektur von Hand' : s.edits ? `${s.edits} Korrekturen von Hand` : '',
			s.clips === 1 ? '1 Clip' : s.clips ? `${s.clips} Clips` : ''
		]
			.filter(Boolean)
			.join(' und ');
	}

	/** Der Editor öffnet die Aufnahme, die hier zuletzt gewählt wurde. */
	function imEditor(/** @type {string} */ stem) {
		try {
			localStorage.setItem('ed:stem', stem);
		} catch {
			// ohne Speicher öffnet der Editor die zuletzt dort gewählte Aufnahme
		}
	}

	/** @param {Auftrag} j */
	function laufzeit(j) {
		if (!j.started) return '';
		return dauer((j.finished || jetzt) - j.started);
	}

	// eine Meldung geht nach einer Weile von selbst, damit sie nicht über der Liste stehen bleibt
	$effect(() => {
		if (!meldung) return;
		const t = setTimeout(() => (meldung = ''), 12000);
		return () => clearTimeout(t);
	});

	// Nach dem Ablegen von Dateien im Fenster (Einrichtung.svelte) oder einer eigenen Aktion sofort neu laden
	$effect(() => {
		auftraege.stand;
		// nur auftraege.stand löst aus: was die beiden lesen (laufend, die Aufnahmen), ändern sie selbst
		untrack(() => {
			ladeAufnahmen();
			ladeAuftraege();
		});
	});

	onMount(() => {
		ladeOrdner();
		vorgaben()
			.then(({ dropMode: _, erkennung: e, ...o }) => {
				optionen = o;
				erkennung = e;
			})
			.catch(() => {});
		const t = setInterval(() => {
			ladeAuftraege();
			jetzt = Date.now() / 1000;
		}, 2000);
		// neue Aufnahmen in den Ordnern, und was andere dort transkribiert oder korrigiert haben
		const nachsehen = einzeln(ladeAufnahmen);
		const o = setInterval(() => {
			if (ordner.folders.length && !document.hidden) nachsehen();
		}, 10000);
		return () => {
			clearInterval(t);
			clearInterval(o);
			clearTimeout(nachProbe);
		};
	});
</script>

<svelte:head><title>Bibliothek · Transcribe by Moliri</title></svelte:head>

{#snippet bereichsknopf(/** @type {string} */ b, /** @type {string} */ name, /** @type {any} */ Icon, /** @type {string} */ hinweis)}
	{@const z = zahlen[b]}
	<li>
		<button type="button" class="bereich" aria-current={bereich === b ? 'page' : undefined} title={hinweis || undefined} onclick={() => zeige(b)}>
			<Icon aria-hidden="true" />
			<span class="bname">{name}</span>
			{#if z?.offen}<span class="offen" title="{z.offen} ohne Transkript">{z.offen}</span>{/if}
			<span class="bzahl">{z?.alle ?? 0}</span>
		</button>
	</li>
{/snippet}

<!-- vorn in jeder Zeile: anhören, in Signalgrün, solange sie spielt -->
{#snippet abspielknopf(/** @type {Aufnahme} */ a)}
	{@const laeuft = spielt?.name === a.name && !pausiert}
	<button
		class="anhoeren"
		class:laeuft
		type="button"
		title={laeuft ? 'Anhalten' : 'Anhören'}
		aria-label="{a.title} {laeuft ? 'anhalten' : 'anhören'}"
		onclick={() => anhoeren(a)}
	>
		{#if laeuft}<IconMaterialSymbolsPauseRounded aria-hidden="true" />{:else}<IconMaterialSymbolsPlayArrowRounded aria-hidden="true" />{/if}
	</button>
{/snippet}

<!-- je Aufnahme: Details, Name und Schlagworte, dann Löschen oder, wo Transcribe nichts löscht
     (verbundene Ordner), Ausblenden; unter „Ausgeblendet“ statt beidem Einblenden -->
{#snippet knoepfe(/** @type {Aufnahme} */ a)}
	<button class="neu-tx" type="button" title="Details zur Datei" aria-label="Details zu {a.title}" onclick={() => zeigeDetails(a)}>
		<IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" />
	</button>
	<button class="neu-tx" type="button" title="Name und Schlagworte" aria-label="Name und Schlagworte von {a.title}" onclick={() => fragBeschriften(a)}>
		<IconMaterialSymbolsSellOutline aria-hidden="true" />
	</button>
	{#if a.hidden}
		<button class="neu-tx" type="button" title="Wieder einblenden" aria-label="{a.title} wieder einblenden" onclick={() => ausblenden(a, false)}>
			<IconMaterialSymbolsVisibilityOutlineRounded aria-hidden="true" />
		</button>
	{:else if a.source === 'folder'}
		<button
			class="neu-tx"
			type="button"
			title="Ausblenden; die Datei im Ordner bleibt"
			aria-label="{a.title} ausblenden"
			onclick={() => ausblenden(a, true)}
		>
			<IconMaterialSymbolsVisibilityOffOutlineRounded aria-hidden="true" />
		</button>
	{:else}
		<button
			class="weg"
			type="button"
			disabled={inBearbeitung(a)}
			title={inBearbeitung(a) ? 'Wird gerade transkribiert; erst abbrechen, dann löschen' : 'Löschen'}
			aria-label="{a.title} löschen"
			onclick={() => fragLoeschen(a)}
		>
			<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />
		</button>
	{/if}
{/snippet}

<main>
	<header class="kopf">
		<div class="kopftext">
			<h1>Bibliothek</h1>
			<p>Deine Aufnahmen und ihre Transkripte. Neue Dateien und ganze Ordner kannst du auch einfach ins Fenster ziehen.</p>
		</div>
		<label class="hinzufuegen" class:laedt={laedtHoch}>
			{#if laedtHoch}<IconEosIconsLoading aria-hidden="true" />{:else}<IconMaterialSymbolsUploadRounded aria-hidden="true" />{/if}
			Dateien hinzufügen
			<input class="sr" type="file" accept="audio/*,video/*" multiple onchange={hochladen} />
		</label>
	</header>

	<p class="meldung" class:voll={!!meldung} role="status">
		{meldung}
		{#if zurueck && zurueck.meldung === meldung}
			<button type="button" class="zurueck" onclick={() => zurueck?.tun()}>Rückgängig</button>
		{/if}
	</p>

	{#if offline}
		<p class="leer fehler" role="alert">
			Keine Verbindung zu Transcribe. Läuft das Programm noch?
			{#if laufend.length}Wurde es beendet, ist auch die Transkription stehen geblieben: Nach einem Neustart lässt sie sich mit
				„Erneut“ wieder anstoßen.{/if}
		</p>
	{/if}

	{#if ordner.hint}
		{@const h = ordner.hint}
		<aside class="vorschlag" aria-labelledby="h-vorschlag">
			<IconMaterialSymbolsCreateNewFolderOutlineRounded aria-hidden="true" />
			<div class="vtext">
				<h2 id="h-vorschlag">
					Neben Transcribe {h.count === 1 ? 'liegt eine Aufnahme' : `liegen ${h.count >= 1000 ? 'viele' : h.count} Aufnahmen`}
				</h2>
				<p>
					Im Ordner „{h.name}“{h.count === 1 ? '' : ' und seinen Unterordnern'}. Verbunden steht {h.count === 1 ? 'sie' : 'alles'}
					hier, ohne kopiert zu werden, und die Transkripte liegen gleich daneben.
				</p>
			</div>
			<div class="vtun">
				<Button type="primary" size="large" onclick={() => verbinde(h.path)}>Ordner verbinden</Button>
				<Button type="tertiary" size="large" onclick={() => ordnerAendern('decline', h.path)}>Nein, danke</Button>
			</div>
		</aside>
	{/if}

	<div class="rahmen">
		<nav class="bereiche" aria-label="Bereiche der Bibliothek">
			<ul>
				{@render bereichsknopf('alle', 'Alle Aufnahmen', IconMaterialSymbolsLibraryMusicOutlineRounded, '')}
				{@render bereichsknopf('upload', 'Hochgeladen', IconMaterialSymbolsUploadFileOutlineRounded, 'Ins Fenster gezogen oder hinzugefügt; liegen im Datenordner von Transcribe')}
				{#if zahlen.live?.alle}
					{@render bereichsknopf('live', 'Live-Aufnahmen', IconMaterialSymbolsMicOutlineRounded, 'Auf der Seite Live gespeicherte Aufnahmen')}
				{/if}
			</ul>
			{#if schlagworte.length}
				<h2 class="bkopf">Schlagworte</h2>
				<ul>
					{#each schlagworte as t (t)}
						{@render bereichsknopf('tag:' + t, t, IconMaterialSymbolsSellOutline, '')}
					{/each}
				</ul>
			{/if}
			<h2 class="bkopf">Ordner</h2>
			{#if ordner.folders.length}
				<ul>
					{#each ordner.folders as o (o.path)}
						{@render bereichsknopf(o.path, o.name, o.exists ? IconMaterialSymbolsFolderOutlineRounded : IconMaterialSymbolsFolderOffOutlineRounded, o.exists ? o.path : `${o.path} (nicht erreichbar)`)}
					{/each}
				</ul>
			{/if}
			<button type="button" class="bereich verbinden" aria-haspopup="dialog" onclick={oeffneVerbinden}>
				<IconMaterialSymbolsCreateNewFolderOutlineRounded aria-hidden="true" />
				<span class="bname">Ordner verbinden</span>
			</button>
			{#if zahlen.versteckt?.alle}
				<ul class="unten">
					{@render bereichsknopf('versteckt', 'Ausgeblendet', IconMaterialSymbolsVisibilityOffOutlineRounded, 'Aufnahmen, die du ausgeblendet hast; gelöscht ist nichts')}
				</ul>
			{/if}
		</nav>

		<div class="inhalt">
			{#if imOrdner}
				<div class="ordnerkopf">
					<div>
						<h2>{imOrdner.name}</h2>
						<p class="ordnerpfad">{imOrdner.path}</p>
						<p class="ordnerinfo">
							{#if !imOrdner.exists}
								<span class="fehler">Der Ordner ist gerade nicht erreichbar.</span>
							{:else if ordner.sidecar}
								<IconMaterialSymbolsDescriptionOutlineRounded aria-hidden="true" />
								Transkripte liegen als Datei neben den Aufnahmen, für alle, die den Ordner teilen.
							{:else}
								Transkripte bleiben nur in Transcribe (Einstellungen, Ordner).
							{/if}
						</p>
					</div>
					<Button type="tertiary" size="large" onclick={() => trennen(imOrdner)}>
						{#snippet iconLeft()}<IconMaterialSymbolsLinkOffRounded />{/snippet}
						Ordner trennen
					</Button>
				</div>
			{/if}

			{#if bereich === 'versteckt'}
				<div class="ordnerkopf">
					<div>
						<h2>Ausgeblendet</h2>
						<p class="ordnerinfo">Diese Aufnahmen stehen in keinem anderen Bereich. Gelöscht ist nichts, auch nicht ihr Transkript.</p>
					</div>
				</div>
			{:else if bereich.startsWith('tag:')}
				<div class="ordnerkopf">
					<div>
						<h2><IconMaterialSymbolsSellOutline aria-hidden="true" />{bereich.slice(4)}</h2>
						<p class="ordnerinfo">Alle Aufnahmen mit diesem Schlagwort.</p>
					</div>
				</div>
			{/if}

			{#if inArbeit.length}
				<section class="blatt" aria-labelledby="h-arbeit">
					<div class="zeile">
						<h2 id="h-arbeit">In Arbeit</h2>
						{#if laufend.length}<span class="zaehler">{laufend.length}</span>{/if}
						<span class="platz"></span>
						{#if laufend.length && !offline}
							<Button type="secondary" size="large" disabled={schaltet} onclick={pause}>
								{#snippet iconLeft()}{#if angehalten}<IconMaterialSymbolsPlayArrowRounded />{:else}<IconMaterialSymbolsPauseRounded />{/if}{/snippet}
								{angehalten ? 'Fortsetzen' : 'Pausieren'}
							</Button>
						{/if}
						{#if laufend.length > 1}
							<Button type="secondary" size="large" onclick={alleAbbrechen}>Alle abbrechen</Button>
						{/if}
					</div>
					{#if laufend.length && !offline && (angehalten || liveHaelt)}
						<p class="pausenhinweis" role="status">
							<IconMaterialSymbolsPauseCircleOutlineRounded aria-hidden="true" />
							{#if angehalten}
								Pausiert, bis du fortsetzt, auch über einen Neustart. Die laufende Transkription bleibt an ihrer Stelle stehen{liveHaelt ? '; gerade läuft auch eine Live-Transkription' : ''}.
							{:else}
								Pausiert, solange eine Live-Transkription läuft. Danach geht es von selbst an derselben Stelle weiter.
							{/if}
						</p>
					{/if}
					<ul class="auftraege" aria-label="Aufträge">
						{#each inArbeit as j (j.id)}
							{@const pct = Math.round((j.progress || 0) * 100)}
							{@const laeuft = LAUFEND.includes(j.status)}
							<li class="auftrag">
								<div class="kopfzeile">
									<div class="was">
										<span class="name">{titel(j.audio)}</span>
										{#if offline && laeuft}
											<span class="status unbekannt">unbekannt</span>
										{:else}
											<span class="status {j.status}">{STATUS[j.status] || j.status}</span>
										{/if}
										<span class="meta">{wie(j)}{#if j.started && !(offline && laeuft)}&nbsp;· <span class="zeit">{laufzeit(j)}</span>{/if}</span>
									</div>
									<div class="tun">
										{#if laeuft}
											<Button type="secondary" size="large" disabled={offline || j.status === 'canceling'} onclick={() => aktion(j, 'cancel')}>Abbrechen</Button>
										{:else}
											<Button type="secondary" size="large" onclick={() => aktion(j, 'retry')}>
												{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
												Erneut
											</Button>
											<Button type="tertiary" size="large" onclick={() => aktion(j, 'remove')}>Entfernen</Button>
										{/if}
									</div>
								</div>
								{#if (j.status === 'running' || j.status === 'paused') && !offline}
									<div class="balken" class:ruht={j.status === 'paused'} role="progressbar" aria-label="Fortschritt {titel(j.audio)}" aria-valuenow={pct} aria-valuemin="0" aria-valuemax="100">
										<i style:width="{pct}%"></i>
									</div>
									<p class="schritt">{j.step || 'startet …'} ({pct} %)</p>
								{/if}
								{#if j.note}<p class="schritt">{j.note}</p>{/if}
								{#if j.error}<p class="fehler">{j.error}</p>{/if}
								{#if j.log?.length && j.status === 'error'}
									<details>
										<summary>Protokoll</summary>
										<pre>{j.log.join('\n')}</pre>
									</details>
								{/if}
							</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if neu.length}
				<section class="blatt" aria-labelledby="h-neu">
					<div class="zeile">
						<h2 id="h-neu">Noch nicht transkribiert</h2>
						<span class="anzahl">{neu.length}</span>
					</div>
					<div class="aufnahmen" role="group" aria-label="Aufnahmen ohne Transkript">
						{#each neu as a (a.name)}
							{@const wo = ort(a)}
							{@const [wannEntstanden, wasEntstanden] = entstanden(a)}
							<div class="aufnahme" class:gewaehlt={gewaehlt.includes(a.name)}>
								{@render abspielknopf(a)}
								<label title={a.path}>
									<input type="checkbox" value={a.name} bind:group={gewaehlt} />
									<span class="name">{a.title !== a.file ? a.title : a.source === 'folder' ? a.name.replace(/^~[0-9a-f]{10}~/, '') : a.name}</span>
									{#if wo}<span class="wo">{wo}</span>{/if}
									{#each a.tags as t (t)}<span class="marke">{t}</span>{/each}
									{#if a.video}
										<span class="art" title="Video"><IconMaterialSymbolsMovieOutlineRounded aria-hidden="true" /><span class="sr">Video</span></span>
									{/if}
									<span class="eckdaten">
										<span class="laenge">{a.media ? dauer(a.media.duration) : ''}</span>
										<span class="format" title={formatInWorten(a)}>{endung(a.name)}</span>
										<span class="groesse">{a.size ? groesse(a.size) : ''}</span>
										<span class="datum" title="{wasEntstanden} am {datum(wannEntstanden)}">{wann(wannEntstanden)}</span>
									</span>
								</label>
								{@render knoepfe(a)}
							</div>
						{/each}
					</div>
					<AuftragsOptionen bind:optionen {erkennung} />
					<div class="handeln">
						<Button type="primary" size="large" loading={reiht} onclick={transkribieren}>
							{#snippet iconLeft()}<IconMaterialSymbolsGraphicEqRounded />{/snippet}
							{gewaehlt.length > 1 ? `${gewaehlt.length} Aufnahmen transkribieren` : 'Transkribieren'}
						</Button>
						{#if neu.length > 1}
							<button
								class="alle"
								type="button"
								onclick={() => (gewaehlt = gewaehlt.length === neu.length ? [] : neu.map((a) => a.name))}
							>
								{gewaehlt.length === neu.length ? 'Keine auswählen' : 'Alle auswählen'}
							</button>
						{/if}
					</div>
				</section>
			{/if}

			<section class="regal" aria-labelledby="h-fertig">
				<div class="regalkopf">
					<h2 id="h-fertig">Transkribiert</h2>
					{#if fertig.length}<span class="anzahl">{fertig.length}</span>{/if}
					<span class="platz"></span>
					{#if fertig.length > 3}
						<label class="suche">
							<IconMaterialSymbolsSearchRounded aria-hidden="true" />
							<span class="sr">In Transkripten suchen</span>
							<input class="feld" type="search" placeholder="Name, Sprecher oder Text" bind:value={suche} />
						</label>
					{/if}
					{#if fertig.length > 1}
						<label class="sortieren">
							<span class="sr">Sortieren nach</span>
							<select class="feld" bind:value={sorte}>
								{#each SORTEN as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
							</select>
						</label>
					{/if}
				</div>

				{#if aufnahmen === null}
					<p class="leer">Lade die Bibliothek …</p>
				{:else if !aufnahmen.length}
					<div class="willkommen">
						<IconMaterialSymbolsLibraryMusicOutlineRounded aria-hidden="true" />
						<strong>Noch keine Aufnahmen</strong>
						<span>
							Zieh Audio- oder Videodateien in dieses Fenster, wähle sie aus, oder verbinde einen Ordner, in dem sie liegen.
							Transcribe schreibt mit, wer was sagt.
						</span>
						<div class="wkn">
							<label class="knopf">
								Dateien auswählen
								<input class="sr" type="file" accept="audio/*,video/*" multiple onchange={hochladen} />
							</label>
							<button type="button" class="knopf hell" onclick={oeffneVerbinden}>Ordner verbinden</button>
						</div>
					</div>
					<!-- wer Transcribe neu hat: woher es kommt und dass es auch ohne Moliri ganz da ist -->
					<aside class="moliri" aria-labelledby="h-moliri">
						<MoliriLogo class="moliri-zeichen" />
						<div>
							<h3 id="h-moliri">Transcribe by Moliri</h3>
							<p>
								Transcribe läuft für sich auf deinem Rechner, ohne Konto, und deine Aufnahmen bleiben hier. Es ist
								auch die Begleit-App zur Museumsdatenbank Moliri: Verbunden diktierst du in Moliris Textfelder, und
								Transcribe liest den Text aus Scans.
							</p>
							<ul class="moliri-wege">
								<li><a href={HANDBUCH} target="_blank" rel="noopener">Handbuch lesen</a></li>
								<li><a href={MIT_MOLIRI} target="_blank" rel="noopener">Mit Moliri verbinden</a></li>
								<li><a href={MOLIRI} target="_blank" rel="noopener">Moliri kennenlernen</a></li>
							</ul>
						</div>
					</aside>
				{:else if !fertig.length}
					<p class="leer">
						{imBereich.length ? 'Hier ist noch kein Transkript fertig. Was du oben transkribierst, steht danach hier.' : 'Hier liegen keine Aufnahmen.'}
					</p>
				{:else if !regal.length}
					<p class="leer">Nichts gefunden zu „{suche}“.</p>
				{:else}
					<ul class="liste" aria-label="Transkribierte Aufnahmen">
						{#each regal as a (a.name)}
							{@const s = /** @type {Inhalt} */ (a.summary)}
							{@const j = letzter.get(a.name)}
							{@const arbeitet = !!j && LAUFEND.includes(j.status)}
							{@const erster = s.speakers[0]}
							{@const wo = ort(a)}
							{@const [wannEntstanden, wasEntstanden] = entstanden(a)}
							<li class="eintrag" class:arbeitet>
								{@render abspielknopf(a)}
								<div class="haupt">
									<h3 class="titel">
										<a href="/editor" data-sveltekit-reload onclick={() => imEditor(a.stem)} title="{a.title} im Editor öffnen">{a.title}</a>
									</h3>
									{#if wo}<span class="wo">{wo}</span>{/if}
									{#each a.tags as t (t)}
										<button type="button" class="marke" title="Alle mit diesem Schlagwort" onclick={() => zeige('tag:' + t)}>{t}</button>
									{/each}
									{#if a.sidecar}
										<span class="neben" title="Das Transkript liegt als Datei neben der Aufnahme">
											<IconMaterialSymbolsDescriptionOutlineRounded aria-hidden="true" /><span class="sr">Transkript neben der Aufnahme</span>
										</span>
									{/if}
									{#if arbeitet}
										<span class="status running">wird neu transkribiert</span>
									{:else if frisch(a)}
										<span class="status done">neu</span>
									{/if}
									<p class="auszug">
										{#if erster}<span class="wer" style:--c="var(--spk{erster.id % 8})">{erster.name}</span>{/if}
										{s.preview || 'Kein Text erkannt.'}
									</p>
								</div>
								<ul class="stimmen" aria-label="Sprecher">
									{#each s.speakers.slice(0, 3) as p (p.id)}
										<li><span class="punkt" style:--c="var(--spk{p.id % 8})" aria-hidden="true"></span>{p.name}</li>
									{/each}
									{#if s.speakers.length > 3}<li class="mehr">+{s.speakers.length - 3}</li>{/if}
								</ul>
								<p class="stand">
									{#if a.video}<IconMaterialSymbolsMovieOutlineRounded aria-label="Video" />{/if}
									<span class="zeit">{dauer(s.duration)} <span class="format" title={formatInWorten(a)}>{endung(a.name)}</span></span>
									<span class="sr">,</span>
									<span class="leise">{s.speakers.length === 1 ? '1 Sprecher' : `${s.speakers.length} Sprecher`}</span>
								</p>
								<p class="arbeit">
									{#if s.edits}<span title="Von Hand korrigiert"><IconMaterialSymbolsEditOutlineRounded aria-hidden="true" />{s.edits === 1 ? '1 Korrektur' : `${s.edits} Korrekturen`}</span>{/if}
									{#if s.clips}<span title="Clips"><IconMaterialSymbolsContentCutRounded aria-hidden="true" />{s.clips === 1 ? '1 Clip' : `${s.clips} Clips`}</span>{/if}
								</p>
								<p class="wann" title="Transkribiert am {datum(s.done)}, {wasEntstanden.toLowerCase()} am {datum(wannEntstanden)}">{wann(s.done)}</p>
								<div class="knoepfe-zeile">
									<button
										class="neu-tx"
										type="button"
										disabled={arbeitet}
										title="Neu transkribieren; Korrekturen von Hand bleiben erhalten"
										aria-label="{a.title} neu transkribieren"
										onclick={() => neuTranskribieren(a)}
									>
										<IconMaterialSymbolsRefreshRounded aria-hidden="true" />
									</button>
									{@render knoepfe(a)}
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>
	</div>

	{#if spielt}
		<div class="leiste" role="region" aria-label="Wiedergabe">
			<div class="spielt">
				<strong>{spielt.title}</strong>
				{#if spielt.title !== spielt.file}<span class="leise">{spielt.file}</span>{/if}
			</div>
			<Abspielen bind:this={spieler} bind:pause={pausiert} datei={spielt.name} titel={spielt.title} sofort />
			<button type="button" class="zu" aria-label="Wiedergabe schließen" title="Schließen" onclick={() => (spielt = null)}>
				<IconMaterialSymbolsCloseRounded aria-hidden="true" />
			</button>
		</div>
	{/if}

	<Beschriften bind:this={beschrifter} vorhanden={alleSchlagworte} ondone={beschriftet} />
	<Dateiinfo bind:this={details} oneditor={(a) => imEditor(a.stem)} />

	<dialog class="verbinden-dialog" bind:this={verbindenDialog} aria-labelledby="h-verbinden" onclose={() => (verbinden = false)}>
		<div class="zeile">
			<h2 id="h-verbinden">Ordner verbinden</h2>
			<span class="platz"></span>
			<button type="button" class="zu" aria-label="Schließen" onclick={schliesseVerbinden}><IconMaterialSymbolsCloseRounded /></button>
		</div>
		<p class="erklaert">
			Alle Aufnahmen im Ordner und in seinen Unterordnern stehen dann hier, ohne kopiert zu werden. Neue kommen von selbst
			dazu. {#if ordner.sidecar}Jedes Transkript legt Transcribe als Datei neben seine Aufnahme („Interview.mp3.transcribe.json“):
				Wer den Ordner mit dir teilt, etwa über Nextcloud, sieht dieselben Transkripte und Korrekturen.{:else}Transkripte
				bleiben nur in Transcribe; neben die Aufnahmen legen lässt es sich in den Einstellungen.{/if}
		</p>
		<div class="wege">
			{#if ordner.pick}
				<Button type="primary" size="large" loading={waehlt} onclick={waehlen}>
					{#snippet iconLeft()}<IconMaterialSymbolsFolderOpenOutlineRounded />{/snippet}
					Ordner wählen …
				</Button>
				<span class="oder">oder</span>
			{/if}
			<div class="pfad">
				<label class="sr" for="pfad">Pfad des Ordners</label>
				<input
					id="pfad"
					class="feld"
					type="text"
					placeholder="Pfad einfügen, etwa C:\Nextcloud\Interviews"
					bind:value={pfad}
					onkeydown={(e) => e.key === 'Enter' && verbinde(pfad)}
				/>
				<Button type="secondary" size="large" onclick={() => verbinde(pfad)}>Verbinden</Button>
			</div>
		</div>
		{#if verbindenFehler}<p class="fehler" role="alert">{verbindenFehler}</p>{/if}
	</dialog>

	<dialog class="rueckfrage" bind:this={rueckfrage} aria-labelledby="h-loeschen" onclose={() => (loeschen = null)}>
		{#if loeschen}
			{@const s = loeschen.summary}
			<h2 id="h-loeschen">„{loeschen.title}“ löschen?</h2>
			{#if s}
				<p>
					Die Aufnahme wird von diesem Computer gelöscht, und mit ihr das Transkript samt allen Korrekturen, Sprechernamen und
					Clips.
				</p>
				{#if s.edits || s.clips}
					<p>Darin stecken {arbeitInWorten(s)}.</p>
				{/if}
			{:else}
				<p>Die Datei wird aus der Bibliothek und von diesem Computer gelöscht.</p>
			{/if}
			{#if letzter.get(loeschen.name)?.status === 'queued'}
				<p>Sie wartet gerade darauf, transkribiert zu werden; der Auftrag wird abgebrochen.</p>
			{/if}
			<p><strong>Das lässt sich nicht rückgängig machen.</strong></p>
			<div class="knoepfe">
				<Button type="secondary" size="large" autofocus onclick={() => rueckfrage.close()}>Abbrechen</Button>
				<button class="endgueltig" type="button" disabled={loescht} onclick={loeschenBestaetigt}>
					<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />
					{s ? 'Aufnahme und Transkript löschen' : 'Aufnahme löschen'}
				</button>
			</div>
		{/if}
	</dialog>
</main>

<style>
	main {
		max-width: 84rem;
		margin: 0 auto;
		padding: 2rem 1.5rem 4rem;
		display: grid;
		gap: 1.5rem;
	}

	.kopf {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem 2rem;
	}
	.kopf h1 {
		font-size: 2rem;
		letter-spacing: -0.01em;
		margin-bottom: 0.4rem;
	}
	.kopf p {
		color: hsl(var(--color-text-secondary));
		max-width: 60ch;
	}

	/* Hinzufügen: der Hauptweg in die Bibliothek, als Tintenpille wie Moliris primärer Knopf */
	.hinzufuegen {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		min-height: 3rem;
		padding: 0 1.4rem 0 1.1rem;
		border-radius: 2rem;
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
		font-weight: 600;
		letter-spacing: 0.05em;
		cursor: pointer;
		transition: background-color 0.2s;
	}
	.hinzufuegen:hover {
		background: color-mix(in oklab, hsl(var(--color-neutral-100)) 80%, hsl(var(--color-neutral-0)));
	}
	.hinzufuegen:focus-within {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.hinzufuegen :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}
	.hinzufuegen.laedt {
		pointer-events: none;
		opacity: 0.7;
	}

	.meldung {
		min-height: 1.5em;
		margin: -1rem 0 -0.5rem;
		color: hsl(var(--color-text-secondary));
	}
	.meldung:empty {
		margin-bottom: -1.5rem;
	}
	/* eine Meldung klebt unter der App-Leiste: wer weiter unten etwas tut (ausblenden), sieht sie
	   samt „Rückgängig“, ohne hochzuscrollen */
	.meldung.voll {
		position: sticky;
		top: 4.5rem;
		z-index: 20;
		justify-self: center;
		max-width: 100%;
		padding: 0.5rem 1.125rem;
		border-radius: 1.5rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}

	.blatt {
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		padding: 1.5rem 1.75rem 1.75rem;
		box-shadow: 0 1px 2px hsl(var(--color-shadow)), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
		display: grid;
		gap: 1.25rem;
	}
	h2 {
		font-size: 1.25rem;
	}
	.zeile,
	.regalkopf {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 0.75rem;
	}
	.platz {
		flex: 1;
	}
	.zaehler {
		min-width: 1.5rem;
		padding: 0.1rem 0.5rem;
		border-radius: 999px;
		background: hsl(var(--color-green-100));
		font-size: 0.875rem;
		font-weight: 700;
		text-align: center;
		font-variant-numeric: tabular-nums;
	}
	.anzahl {
		color: hsl(var(--color-text-tertiary));
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.leer {
		color: hsl(var(--color-text-secondary));
		padding: 0.5rem 0;
	}
	.fehler {
		color: hsl(var(--color-text-error));
		font-weight: 600;
	}

	/* ---- Bereiche: links wie das Menü der Einstellungen, rechts ihr Inhalt ---- */
	.rahmen {
		display: grid;
		grid-template-columns: 15rem minmax(0, 1fr);
		gap: 2rem;
		align-items: start;
	}
	.inhalt {
		display: grid;
		gap: 1.5rem;
		min-width: 0;
	}
	.bereiche {
		position: sticky;
		top: calc(60px + 1.5rem);
		display: grid;
		gap: 0.25rem;
	}
	.bereiche ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.125rem;
	}
	.bkopf {
		margin: 1rem 0 0.25rem 1rem;
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: hsl(var(--color-text-tertiary));
	}
	.bereich {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-height: var(--hit);
		padding: 0 0.9rem;
		border-radius: 2rem;
		font-weight: 600;
		text-align: left;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}
	.bereich:hover {
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
	}
	.bereich :global(svg) {
		flex: none;
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}
	/* wo man ist: Tintenpille wie der aktive Tab der App-Leiste, umgekehrt */
	.bereich[aria-current='page'] {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	.bname {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.bzahl {
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		opacity: 0.75;
	}
	/* ohne Transkript: eine grüne Zahl, die zum Transkribieren einlädt */
	.offen {
		min-width: 1.4rem;
		padding: 0 0.4rem;
		border-radius: 999px;
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
		font-size: 0.8125rem;
		font-weight: 700;
		text-align: center;
		font-variant-numeric: tabular-nums;
	}
	.verbinden {
		margin-top: 0.25rem;
		color: hsl(var(--color-text-primary));
	}
	/* „Ausgeblendet“ abgesetzt unter allem: ein Bereich, in den man selten schaut */
	.unten {
		margin-top: 1.25rem;
		padding-top: 0.75rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}

	/* ---- Ordner verbinden ---- */
	.verbinden-dialog {
		width: min(44rem, calc(100vw - 2rem));
		margin: auto;
		padding: 1.5rem 1.75rem 1.75rem;
		border: 0;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow: 0 24px 64px -24px hsl(var(--color-neutral-100) / 0.5);
	}
	.verbinden-dialog[open] {
		display: grid;
		gap: 1.25rem;
	}
	.verbinden-dialog::backdrop {
		background: hsl(var(--color-neutral-100) / 0.3);
		backdrop-filter: blur(4px);
	}
	.zu {
		width: var(--hit);
		height: var(--hit);
		display: grid;
		place-items: center;
		margin: -0.5rem -0.75rem -0.5rem 0;
		border-radius: 50%;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
	}
	.zu:hover {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	.erklaert {
		max-width: 64ch;
		color: hsl(var(--color-text-secondary));
	}
	.wege {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1rem;
	}
	.oder {
		color: hsl(var(--color-text-tertiary));
	}
	.pfad {
		flex: 1 1 24rem;
		display: flex;
		gap: 0.5rem;
	}
	.pfad .feld {
		flex: 1;
		min-width: 0;
	}

	.ordnerkopf {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 0.5rem 1.5rem;
	}
	.ordnerkopf h2 {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1.5rem;
		letter-spacing: -0.01em;
		overflow-wrap: anywhere;
	}
	.ordnerkopf h2 :global(svg) {
		flex: none;
		width: 1.5rem;
		height: 1.5rem;
		color: hsl(var(--color-text-secondary));
	}
	.ordnerpfad {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-tertiary));
		overflow-wrap: anywhere;
	}
	.ordnerinfo {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		margin-top: 0.35rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.ordnerinfo :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
		margin: 0;
	}

	/* ---- Vorschlag: der Ordner neben der EXE ---- */
	.vorschlag {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 1rem 1.25rem;
		padding: 1.25rem 1.5rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-green-10));
		box-shadow: inset 0 0 0 1px hsl(var(--color-green-100) / 0.5);
	}
	.vorschlag > :global(svg) {
		flex: none;
		width: 2rem;
		height: 2rem;
		margin: 0;
	}
	.vtext {
		flex: 1 1 24rem;
	}
	.vtext h2 {
		font-size: 1.125rem;
	}
	.vtext p {
		color: hsl(var(--color-text-secondary));
	}
	.vtun {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	/* ---- In Arbeit ---- */
	.auftraege {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.auftrag {
		display: grid;
		gap: 0.6rem;
		padding: 1rem 0;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	.auftrag:first-child {
		border-top: 0;
		padding-top: 0;
	}
	.auftrag:last-child {
		padding-bottom: 0;
	}
	.kopfzeile {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 1rem;
	}
	.was {
		flex: 1 1 18rem;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 0.75rem;
	}
	.tun {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.auftrag .name {
		font-weight: 700;
		overflow-wrap: anywhere;
	}
	.meta,
	.schritt {
		color: hsl(var(--color-text-secondary));
		font-size: 0.9375rem;
	}
	.zeit {
		font-variant-numeric: tabular-nums;
	}

	/* Zustand als Pille: läuft in Signalgrün wie „online“ in Moliri */
	.status {
		padding: 0.1rem 0.65rem;
		border-radius: 999px;
		font-size: 0.875rem;
		font-weight: 700;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-secondary));
		white-space: nowrap;
	}
	.status.running {
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
	}
	.status.done {
		background: hsl(var(--color-green-10));
		color: hsl(var(--color-text-success));
	}
	.status.error {
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
	}
	.status.paused {
		background: hsl(var(--color-orange-10));
		color: hsl(var(--color-text-warning));
	}
	.status.canceling {
		background: hsl(var(--color-orange-10));
		color: hsl(var(--color-text-warning));
	}

	.balken {
		height: 0.5rem;
		border-radius: 999px;
		background: hsl(var(--color-bg-secondary));
		overflow: hidden;
	}
	.balken i {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: hsl(var(--color-green-100));
		transition: width 0.4s ease-out;
	}
	/* pausiert: der Balken steht in Moliris Pausen-Orange */
	.balken.ruht i {
		background: hsl(var(--color-orange-100));
	}
	.pausenhinweis {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin: 0;
		color: hsl(var(--color-text-warning));
	}
	.pausenhinweis :global(svg) {
		flex: none;
		font-size: 1.25rem;
	}
	details summary {
		display: inline-flex;
		align-items: center;
		min-height: 2rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
	}
	details pre {
		margin-top: 0.4rem;
		padding: 0.75rem 1rem;
		max-height: 14rem;
		overflow: auto;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 0.875rem;
		white-space: pre-wrap;
	}

	/* ---- Noch nicht transkribiert: eine Zeile je Datei, die ganze Zeile ist das Klickziel ---- */
	.aufnahmen {
		display: grid;
		max-height: 18rem;
		overflow: auto;
		border: 1px solid hsl(var(--color-border-primary));
		border-radius: 0.5rem;
		padding: 0.25rem;
	}
	.aufnahme {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		border-radius: 0.375rem;
		transition: background-color 0.15s;
	}
	/* in a narrow window size and date go to a line of their own, rather than squeezing the name */
	.aufnahme label {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 0.75rem;
		min-height: var(--hit);
		padding: 0.25rem 0.75rem;
		cursor: pointer;
	}
	.aufnahme:hover {
		background: hsl(var(--color-interactive-hover));
	}
	.aufnahme.gewaehlt {
		background: hsl(var(--color-green-10));
	}
	.aufnahme input {
		width: 1.25rem;
		height: 1.25rem;
		flex: none;
		cursor: pointer;
	}
	.aufnahme .name {
		flex: 1 1 8rem;
		min-width: 0;
		overflow-wrap: anywhere;
	}
	/* where it lies gives way first, so that length, format, size and date stay on the name's line */
	.aufnahme .wo {
		flex: 0 1 auto;
		max-width: 11rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.art {
		display: inline-flex;
		color: hsl(var(--color-text-secondary));
	}
	/* Länge, Format, Größe und Datum in festen Spalten, damit sie untereinander stehen */
	.eckdaten {
		margin-left: auto;
		padding-left: 1rem;
		display: grid;
		grid-template-columns: 3.5rem 3rem 4.25rem 6.5rem;
		gap: 0.6rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-tertiary));
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}
	.eckdaten > span {
		text-align: right;
	}
	.laenge {
		color: hsl(var(--color-text-secondary));
	}
	.format {
		font-size: 0.8125rem;
		letter-spacing: 0.04em;
		color: hsl(var(--color-text-tertiary));
	}
	.aufnahme .anhoeren {
		margin-left: 0.25rem;
	}
	.handeln {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1.25rem;
	}
	.alle {
		min-height: var(--hit);
		padding: 0 0.25rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		cursor: pointer;
	}
	.alle:hover {
		color: hsl(var(--color-text-primary));
	}

	/* ---- Transkribiert: das Regal ---- */
	.regal {
		display: grid;
		gap: 1rem;
		margin-top: 0.5rem;
	}
	.regalkopf h2 {
		font-size: 1.5rem;
		letter-spacing: -0.01em;
	}
	.suche {
		position: relative;
		display: flex;
		align-items: center;
		flex: 0 1 20rem;
	}
	.suche :global(svg) {
		position: absolute;
		left: 0.75rem;
		width: 1.25rem;
		height: 1.25rem;
		color: hsl(var(--color-text-secondary));
		pointer-events: none;
	}
	.suche .feld {
		width: 100%;
		padding-left: 2.5rem;
	}

	/* Eine Zeile je Aufnahme: Titel mit dem Anfang des Transkripts, wer spricht, wie lang, wie viel
	   von Hand getan ist, wann zuletzt. Der Titel ist der Link; er deckt die ganze Zeile ab, nur
	   „Neu transkribieren“ liegt darüber. */
	.liste {
		list-style: none;
		margin: 0;
		padding: 0.25rem 0;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		box-shadow: 0 1px 2px hsl(var(--color-shadow)), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
		/* the columns follow the list's own width, not the window's: beside the areas on the left
		   the list is far narrower than the window */
		container: regal / inline-size;
	}
	.eintrag {
		position: relative;
		display: grid;
		grid-template-columns: var(--hit) minmax(0, 1fr) 11rem 6.5rem 8.5rem 6.5rem calc(4 * var(--hit));
		align-items: center;
		gap: 0.25rem 1rem;
		padding: 0.75rem 0.75rem 0.75rem 0.875rem;
		border-top: 1px solid hsl(var(--color-border-primary));
		transition: background-color 0.15s;
	}
	.eintrag:first-child {
		border-top: 0;
	}
	.eintrag:hover {
		background: hsl(var(--color-interactive-hover));
	}
	.eintrag:has(.titel a:focus-visible) {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: -2px;
	}
	.eintrag.arbeitet {
		box-shadow: inset 3px 0 0 hsl(var(--color-green-100));
	}
	.haupt {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.15rem 0.6rem;
		min-width: 0;
	}
	.titel {
		font-size: 1.0625rem;
		overflow-wrap: anywhere;
	}
	.titel a {
		text-decoration: none;
	}
	.titel a:focus-visible {
		outline: none;
	}
	.titel a::after {
		content: '';
		position: absolute;
		inset: 0;
	}
	.eintrag:hover .titel a {
		text-decoration: underline;
		text-decoration-color: hsl(var(--color-green-100));
		text-decoration-thickness: 2px;
		text-underline-offset: 0.2em;
	}
	.wo {
		font-size: 0.875rem;
		color: hsl(var(--color-text-tertiary));
		overflow-wrap: anywhere;
	}
	/* über dem Link der Zeile, damit sein Hinweis zu sehen ist */
	.neben {
		position: relative;
		z-index: 1;
		display: inline-flex;
		align-self: center;
		color: hsl(var(--color-text-tertiary));
	}
	.neben :global(svg) {
		width: 1.0625rem;
		height: 1.0625rem;
		margin: 0;
	}
	.auszug {
		flex: 1 1 100%;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.wer {
		margin-right: 0.4rem;
		color: var(--c);
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.stimmen {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.1rem;
		font-size: 0.875rem;
		font-weight: 600;
		min-width: 0;
	}
	.stimmen li {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.punkt {
		flex: none;
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--c);
	}
	.mehr {
		color: hsl(var(--color-text-tertiary));
	}
	.stand {
		display: grid;
		font-size: 0.9375rem;
	}
	.stand :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
		margin: 0;
		color: hsl(var(--color-text-secondary));
	}
	.leise,
	.wann {
		font-size: 0.875rem;
		color: hsl(var(--color-text-secondary));
	}
	.arbeit {
		display: grid;
		gap: 0.1rem;
		font-size: 0.875rem;
		color: hsl(var(--color-text-secondary));
	}
	.arbeit span {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		white-space: nowrap;
	}
	.arbeit :global(svg) {
		width: 1.0625rem;
		height: 1.0625rem;
		margin: 0;
	}
	/* über dem Link der Zeile */
	.neu-tx {
		position: relative;
		z-index: 1;
		width: var(--hit);
		height: var(--hit);
		display: grid;
		place-items: center;
		border-radius: 50%;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}
	.neu-tx:hover:not(:disabled) {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	.neu-tx:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.neu-tx :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}
	.knoepfe-zeile {
		display: flex;
		justify-content: flex-end;
	}

	/* Anhören: ein Kreis mit Tintenrand vorn in der Zeile, über ihrem Link; spielt sie, Signalgrün
	   wie der Abspielknopf im Editor */
	.anhoeren {
		position: relative;
		z-index: 1;
		flex: none;
		width: var(--hit);
		height: var(--hit);
		display: grid;
		place-items: center;
		border-radius: 50%;
		box-shadow: inset 0 0 0 1.5px hsl(var(--color-neutral-100) / 0.35);
		color: hsl(var(--color-text-primary));
		cursor: pointer;
		transition: background-color 0.15s, box-shadow 0.15s, transform 0.15s;
	}
	.anhoeren:hover {
		box-shadow: inset 0 0 0 1.5px hsl(var(--color-neutral-100));
		transform: scale(1.05);
	}
	.anhoeren.laeuft {
		background: hsl(var(--color-green-100));
		box-shadow: inset 0 0 0 1px hsl(var(--color-neutral-100) / 0.25);
		color: hsl(var(--color-neutral-100));
	}
	.anhoeren :global(svg) {
		width: 1.625rem;
		height: 1.625rem;
	}

	/* die Abspielleiste: klebt unten, ein Papierband über der Liste wie die Meldung oben */
	.leiste {
		position: sticky;
		bottom: 1rem;
		z-index: 20;
		display: flex;
		align-items: center;
		gap: 0.5rem 1.25rem;
		width: min(56rem, 100%);
		justify-self: center;
		padding: 0.5rem 0.75rem 0.5rem 1.5rem;
		border-radius: 2rem;
		background: hsl(var(--color-bg-primary));
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 18px 40px -12px hsl(var(--color-neutral-100) / 0.35);
	}
	.spielt {
		flex: 0 1 16rem;
		min-width: 0;
		display: grid;
	}
	.spielt > * {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.spielt .leise {
		font-size: 0.875rem;
	}
	.leiste :global(.spieler) {
		flex: 1;
		max-width: none;
	}
	.leiste :global(.fehlt) {
		flex: 1;
	}
	.leiste .zu {
		flex: none;
		margin: 0;
	}

	/* Schlagworte einer Aufnahme: Nebel-Marken wie die der Clips; in der Liste führt eine zu allen mit ihr */
	.marke {
		display: inline-flex;
		align-items: center;
		align-self: center;
		min-height: 1.625rem;
		padding: 0 0.625rem;
		border-radius: 2rem;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-secondary));
		font-size: 0.875rem;
		font-weight: 600;
		white-space: nowrap;
	}
	button.marke {
		position: relative;
		z-index: 1;
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}
	button.marke:hover {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}

	.zurueck {
		margin-left: 0.5rem;
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		cursor: pointer;
	}

	/* Löschen: leise, bis man darauf zeigt; in der Zeile am Ende, über ihrem Link */
	.weg {
		position: relative;
		z-index: 1;
		flex: none;
		width: var(--hit);
		height: var(--hit);
		display: grid;
		place-items: center;
		border-radius: 50%;
		color: hsl(var(--color-text-tertiary));
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}
	.weg:hover:not(:disabled) {
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
	}
	.weg:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.weg :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}

	.rueckfrage {
		width: min(36rem, calc(100vw - 2rem));
		margin: auto;
		padding: 1.5rem 1.75rem;
		border: 0;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow: 0 24px 64px -24px hsl(var(--color-neutral-100) / 0.5);
	}
	.rueckfrage::backdrop {
		background: hsl(var(--color-neutral-100) / 0.3);
		backdrop-filter: blur(4px);
	}
	.rueckfrage h2 {
		margin-bottom: 0.75rem;
		overflow-wrap: anywhere;
	}
	.rueckfrage p {
		margin-bottom: 0.6rem;
		color: hsl(var(--color-text-secondary));
		line-height: 1.5;
	}
	.rueckfrage strong {
		color: hsl(var(--color-text-primary));
	}
	.knoepfe {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.75rem;
		margin-top: 1.25rem;
	}
	.endgueltig {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: 3rem;
		padding: 0 1.25rem 0 1rem;
		border-radius: 2rem;
		background: hsl(var(--color-text-error));
		color: hsl(var(--color-neutral-0));
		font-weight: 600;
		letter-spacing: 0.05em;
		cursor: pointer;
	}
	.endgueltig:hover:not(:disabled) {
		filter: brightness(0.9);
	}
	.endgueltig:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.endgueltig :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}

	/* Leer: das Blatt lädt zum ersten Ablegen ein */
	.willkommen {
		display: grid;
		justify-items: center;
		gap: 0.6rem;
		padding: 3rem 1.5rem;
		border: 2px dashed hsl(var(--color-border-secondary));
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		text-align: center;
	}
	.willkommen :global(svg) {
		width: 2.75rem;
		height: 2.75rem;
		color: hsl(var(--color-text-secondary));
	}
	.willkommen strong {
		font-size: 1.25rem;
	}
	.willkommen > span {
		max-width: 44ch;
		color: hsl(var(--color-text-secondary));
	}
	.knopf {
		margin-top: 0.5rem;
		display: inline-flex;
		align-items: center;
		min-height: var(--hit);
		padding: 0 1.25rem;
		border-radius: 2rem;
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
		font-weight: 600;
		letter-spacing: 0.05em;
	}

	/* unter dem leeren Blatt: Transcribe für sich und als Begleiter von Moliri */
	.moliri {
		display: flex;
		align-items: flex-start;
		gap: 1.25rem;
		margin-top: 1.25rem;
		padding: 1.25rem 1.5rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		box-shadow: inset 0 0 0 1px hsl(var(--color-border-secondary));
	}
	.moliri :global(.moliri-zeichen) {
		width: 2.5rem;
		height: 2.5rem;
		margin-top: 0.125rem;
		color: hsl(var(--color-text-primary));
	}
	.moliri h3 {
		margin: 0 0 0.375rem;
		font-size: 1.0625rem;
	}
	.moliri p {
		max-width: 62ch;
		margin: 0;
		color: hsl(var(--color-text-secondary));
	}
	.moliri-wege {
		display: flex;
		flex-wrap: wrap;
		gap: 0 1.5rem;
		margin: 0.5rem 0 0;
		padding: 0;
		list-style: none;
	}
	.moliri-wege a {
		display: inline-flex;
		align-items: center;
		min-height: var(--hit);
		color: hsl(var(--color-text-primary));
		font-weight: 600;
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}
	.moliri-wege a:hover {
		text-decoration-thickness: 2px;
	}

	.wkn {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.75rem;
		margin-top: 0.5rem;
	}
	.knopf.hell {
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow: inset 0 0 0 2px hsl(var(--color-neutral-100));
	}
	.knopf {
		cursor: pointer;
	}
	.knopf:focus-within,
	.knopf:focus-visible {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}

	/* narrower: first the speakers go, then the remaining columns move beside the title, and in a
	   very narrow list below it; the title always keeps room for a few words per line */
	@container regal (width < 64rem) {
		.eintrag {
			grid-template-columns: var(--hit) minmax(0, 1fr) 6.5rem 8.5rem 6.5rem calc(4 * var(--hit));
		}
		.stimmen {
			display: none;
		}
	}
	@container regal (width < 52rem) {
		.eintrag {
			grid-template-columns: var(--hit) minmax(0, 1fr) auto calc(4 * var(--hit));
		}
		.arbeit,
		.wann {
			display: none;
		}
		.stand {
			text-align: right;
		}
	}
	@container regal (width < 36rem) {
		.eintrag {
			grid-template-columns: var(--hit) minmax(0, 1fr) auto;
			align-items: start;
		}
		.haupt {
			grid-column: 2 / -1;
			align-self: center;
		}
		.stand {
			grid-column: 2;
			align-self: center;
			display: flex;
			align-items: center;
			gap: 0.5rem;
			text-align: left;
			white-space: nowrap;
		}
	}

	@media (max-width: 900px) {
		.rahmen {
			grid-template-columns: minmax(0, 1fr);
			gap: 1rem;
		}
		.bereiche {
			position: static;
			display: flex;
			flex-wrap: wrap;
			gap: 0.25rem;
		}
		.bereiche ul {
			display: flex;
			flex-wrap: wrap;
			gap: 0.25rem;
		}
		.bkopf {
			display: none;
		}
		.bereich {
			width: auto;
		}
		.verbinden {
			margin-top: 0;
		}
	}

	@media (max-width: 640px) {
		main {
			padding: 1.25rem 0.75rem 3rem;
		}
		.blatt {
			padding: 1.25rem 1rem;
		}
		.suche {
			flex: 1 1 100%;
			order: 3;
		}
		.leiste {
			flex-wrap: wrap;
			padding: 0.5rem 0.75rem 0.5rem 1.25rem;
			border-radius: 1.25rem;
		}
		.spielt {
			flex: 1 1 0;
		}
		.leiste :global(.spieler) {
			order: 3;
			flex: 1 1 100%;
		}
	}
</style>

