<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Einstellungen in Kategorien, links ein Menü (schmal eine Leiste oben) mit den Kategorien und unter
     der offenen ihren Abschnitten; zu sehen ist immer eine Kategorie. Transkribieren: wo die Sprache
     erkannt wird (auf diesem Rechner, über eine API oder beides; welche API), Vorgaben für neue
     Aufnahmen (Sprecher und Sprachen, sofort gespeichert), verbundene Ordner. Live und Diktieren: was
     Live aufnimmt, „Meine Stimme“, Diktieren per Tastenkürzel. Programme: wer Transcribe nutzen darf,
     Start mit Windows und Weiterlaufen im Hintergrund, die Adresse der API und was von Anfragen bleibt.
     Texterkennung (OCR): die Erkennungen samt Vorgabe für Programme, der KI-Dienst mit eigenem
     Schlüssel, ein Bild zum Ausprobieren. System: GPU-Beschleunigung, Speicherorte, Modelle und
     Komponenten samt Download, Angaben zum Programm samt Version und Update (internal/update).
     Die Adresse trägt die Kategorie oder einen Abschnitt als #anker, etwa /settings#programme. -->
<script>
	import { onMount, tick } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import AuftragsOptionen from '$lib/AuftragsOptionen.svelte';
	import { einrichtung } from '$lib/Einrichtung.svelte';
	import { ENGINES, einzeln, getJSON, post, sprecherAus, verbindung } from '$lib/api.js';
	import { ladeUpdate, update, updateAktion, versionText, vorherigeVersion } from '$lib/update.svelte.js';
	import AufnahmeWahl from '$lib/live/AufnahmeWahl.svelte';
	import StimmeWahl from '$lib/live/StimmeWahl.svelte';
	import { live, LOESCHEN, uhrzeit } from '$lib/live/stand.svelte.js';
	import MoliriLogo from '$lib/MoliriLogo.svelte';
	import Oberflaechensprache from '$lib/Oberflaechensprache.svelte';
	import { HANDBUCH, MOLIRI } from '$lib/moliri-links.js';
	import { getLocale } from '$lib/moliri/i18n/locale.js';
	import plural from '../../locales/plural.js';

	/** @typedef {{ key: string, path: string, size: number }} Ordner */
	/** @typedef {{ name: string, label: string, path: string, installed: boolean, size: number, state: string,
	 *   done: number, total: number, error?: string, onDemand: boolean, device?: string }} Komponente */
	/** @typedef {{ target: string, done: number, total: number, state: string, error: string }} Umzug */
	/** @typedef {{ dataDir: string, default: string, fixed: boolean, pick: boolean, move: Umzug | null }} Ort */
	/** @typedef {{ dataDir: string, exe: string, log: string, os: string, sherpa: string,
	 *   folders: Ordner[], components: Komponente[], location: Ort }} Stand */
	/** @typedef {{ gpu: { present: boolean, supported: boolean, ready: boolean, name: string, memoryMB: number,
	 *   driver: string, cudaMajor: number, missing: string[], fetch?: number }, installed: boolean, accel: string, active: string,
	 *   restartNeeded: boolean }} Gpu */

	const ORDNER = /** @type {Record<string, string>} */ ({
		data: /* @wc-include */ 'Data folder',
		audio: /* @wc-include */ 'Recordings',
		results: /* @wc-include */ 'Results',
		models: /* @wc-include */ 'Models',
		lib: /* @wc-include */ 'Recognition runtime',
		bin: 'ffmpeg'
	});


	const groesse = (/** @type {number} */ n) =>
		n >= 1073741824
			? (n / 1073741824).toLocaleString(getLocale(), { minimumFractionDigits: 2, maximumFractionDigits: 2 }) + ' GB'
			: n >= 1048576
				? Math.round(n / 1048576) + ' MB'
				: n > 0
					? Math.max(1, Math.round(n / 1024)) + ' KB'
					: '–';
	const prozent = (/** @type {Komponente} */ c) => (c.total ? Math.round((c.done * 100) / c.total) : 0);

	// ---- Erkennung ----
	const ARTEN = [
		[
			'local',
			/* @wc-include */ 'Only on this computer',
			/* @wc-include */ 'The models run here. No recording leaves the computer.'
		],
		[
			'api',
			/* @wc-include */ 'Only through an API',
			/* @wc-include */ 'Recordings go to a service for recognition, on the internet or on your own network. The app downloads no models for this.'
		],
		[
			'both',
			/* @wc-include */ 'Both',
			/* @wc-include */ 'You decide what Live and whole recordings use, and can choose again for each job if you like.'
		]
	];
	/** @typedef {{ id: string, label: string, kind: string, url: string, model: string, keys?: string, noKey?: boolean,
	 *   speakers?: boolean, realtime?: boolean }} Dienst */
	/** @typedef {{ provider: string, url: string, model: string, keySet: boolean, keyHint?: string }} Api */
	/** @typedef {{ recognition: string, liveEngine: string, fileEngine: string, api: Api, apiProviders: Dienst[] }} Erkennung */
	/** @type {Erkennung | null} */
	let erkennung = $state(null);
	let schluessel = $state('');
	let schluesselStand = $state('');
	let speichertSchluessel = $state(false);
	let erkennungFehler = $state('');

	/** @param {Erkennung} p */
	function erkennungAus(p) {
		const { recognition, liveEngine, fileEngine, api, apiProviders } = p;
		erkennung = { recognition, liveEngine, fileEngine, api, apiProviders };
	}

	/** @param {Record<string, unknown>} patch */
	async function erkennungSetzen(patch) {
		erkennungFehler = '';
		const vorher = erkennung?.recognition;
		try {
			erkennungAus(await post('/api/prefs', patch));
		} catch (e) {
			erkennungFehler = /* @wc-include */ `Not saved: ${/** @type {Error} */ (e).message}`;
			return false;
		}
		// dürfen die Modelle wieder genutzt werden, lädt die App, was fehlt; die Leiste zeigt es
		if (vorher === 'api' && erkennung?.recognition !== 'api') {
			einrichtung.neu++;
			ladeStand();
		}
		return true;
	}

	/** @param {Record<string, string>} api */
	async function apiSetzen(api) {
		schluesselStand = '';
		await erkennungSetzen({ api });
	}

	/** @param {string} wert leer: den Schlüssel entfernen */
	async function schluesselSpeichern(wert) {
		speichertSchluessel = true;
		schluesselStand = '';
		if (await erkennungSetzen({ api: { key: wert } })) {
			schluessel = '';
			schluesselStand = wert ? /* @wc-include */ 'Key saved.' : /* @wc-include */ 'Key removed.';
		}
		speichertSchluessel = false;
	}

	// ---- Verbundene Ordner (internal/server/folders.go) ----
	/** @type {{ folders: { path: string, name: string, exists: boolean }[], sidecar: boolean } | null} */
	let verbunden = $state(null);
	let nebenAn = $state(true);
	let ordnerStand = $state('');
	let ordnerFehler = $state(false);

	async function ladeVerbunden() {
		verbunden = await getJSON('/api/folders');
		nebenAn = verbunden?.sidecar ?? true;
	}

	/** @param {{ action: 'remove' | 'sidecar', path?: string, on?: boolean }} body */
	async function verbundenAendern(body) {
		ordnerStand = '';
		ordnerFehler = false;
		try {
			verbunden = await post('/api/folders', body);
			nebenAn = verbunden?.sidecar ?? true;
			ordnerStand =
				body.action === 'remove'
					? /* @wc-include */ 'Disconnected. The recordings and transcripts in the folder stay where they are.'
					: /* @wc-include */ 'Saved.';
		} catch (e) {
			ordnerStand = /* @wc-include */ `Not saved: ${/** @type {Error} */ (e).message}`;
			ordnerFehler = true;
		}
	}

	// ---- Vorgaben ----
	/** @type {import('$lib/api.js').Optionen} */
	let optionen = $state({ speakers: '', language: 'auto' });
	let sofort = $state(false);
	let geladen = $state(false);
	let gespeichert = $state('');
	/** the message in gespeichert is an error */
	let gespeichertFehler = $state(false);
	/** @type {ReturnType<typeof setTimeout> | undefined} */
	let gespeichertUhr;

	async function ladeVorgaben() {
		const p = await getJSON('/api/prefs');
		optionen = { speakers: sprecherAus(p.preset, p.speakers), language: p.language };
		sofort = p.dropMode === 'auto';
		erkennungAus(p);
		ocrDienstAus(p);
		geladen = true;
	}

	/** @param {Record<string, unknown>} patch */
	async function speichern(patch) {
		// leeren, damit „Gespeichert.“ für diese Änderung steht, nicht für die vorige
		clearTimeout(gespeichertUhr);
		gespeichert = '';
		gespeichertFehler = false;
		try {
			await post('/api/prefs', patch);
			gespeichert = /* @wc-include */ 'Saved.';
			gespeichertUhr = setTimeout(() => (gespeichert = ''), 2000);
		} catch (e) {
			gespeichert = /* @wc-include */ `Not saved: ${/** @type {Error} */ (e).message}`;
			gespeichertFehler = true;
		}
	}

	async function optionenSpeichern() {
		if (!geladen) return;
		await tick();
		const zahl = (/** @type {unknown} */ v) => parseInt(String(v ?? ''), 10);
		// die Voreinstellung legte nur die Sprecherzahl fest; die steht jetzt selbst da
		speichern({
			preset: 'auto',
			speakers: zahl(optionen.speakers) || 0,
			language: String(optionen.language ?? '').trim() || 'auto'
		});
	}

	// ---- Speicherorte, Komponenten, Programm ----
	/** @type {Stand | null} */
	let stand = $state(null);
	let offline = $state(false);
	/** Fehler einer Aktion, je Ordner oder Komponente */
	let fehler = $state(/** @type {Record<string, string>} */ ({}));
	/** Komponenten, deren Download gerade angefordert wird */
	let fordert = $state(/** @type {string[]} */ ([]));

	/** nur bei Bedarf geladen: in der Liste erst, wenn sie da sind oder gerade laden */
	const NACH_BEDARF = ['runtime-cuda', 'nvidia'];
	const komponenten = $derived(
		(stand?.components ?? []).filter((c) => !NACH_BEDARF.includes(c.name) || c.installed || c.state === 'downloading')
	);
	const cuda = $derived(stand?.components.find((c) => c.name === 'runtime-cuda') ?? null);
	const nvidia = $derived(stand?.components.find((c) => c.name === 'nvidia') ?? null);
	/** pending: wartet in „GPU einrichten“ auf die NVIDIA-Bibliotheken */
	const imGang = (/** @type {Komponente | null} */ c) => c?.state === 'downloading' || c?.state === 'pending';

	const server = verbindung();

	async function ladeStand() {
		const vorher = nvidia?.state;
		try {
			stand = await getJSON('/api/settings', { signal: AbortSignal.timeout(8000) });
			server.ok();
			offline = false;
			// sind die Bibliotheken fertig, gleich neu prüfen statt mit der gemerkten Antwort
			if (vorher === 'downloading' && nvidia?.state !== 'downloading') ladeGpu(true);
		} catch (e) {
			offline = server.fehler(e);
		}
	}

	// ---- Ort des Datenordners: wählen, prüfen, umziehen (internal/server/location.go) ----
	const ort = $derived(stand?.location ?? null);
	const umzug = $derived(ort?.move ?? null);
	/** der geprüfte neue Ort, der auf „Umziehen“ wartet */
	let ortZiel = $state(/** @type {{ target: string, size: number } | null} */ (null));
	/** der Ort, an den die Daten gerade ziehen; bis die App dort neu gestartet ist */
	let ortUnterwegs = $state('');
	let ortFertig = $state('');
	let ortFehler = $state('');
	let ortEingabe = $state('');
	let ortLaeuft = $state('');
	const gleich = (/** @type {string} */ a, /** @type {string} */ b) => a.toLowerCase() === b.toLowerCase();
	const amStandard = $derived(!!ort && gleich(ort.dataDir, ort.default));

	$effect(() => {
		// neu gestartet: die App liegt jetzt am neuen Ort
		if (ortUnterwegs && ort && gleich(ort.dataDir, ortUnterwegs)) {
			ortFertig = ortUnterwegs;
			ortUnterwegs = '';
		}
	});

	async function ortWaehlen() {
		ortFehler = '';
		ortFertig = '';
		ortLaeuft = 'waehlen';
		try {
			const r = await post('/api/settings/location/pick', {});
			if (r.target) ortZiel = { target: r.target, size: r.size };
		} catch (e) {
			ortFehler = /** @type {Error} */ (e).message;
		} finally {
			ortLaeuft = '';
		}
	}

	/** @param {string} pfad ein leerer Pfad ist der Standardort */
	async function ortPruefen(pfad) {
		ortFehler = '';
		ortFertig = '';
		ortLaeuft = pfad ? 'pruefen' : 'standard';
		try {
			ortZiel = await post('/api/settings/location', { path: pfad, check: true });
		} catch (e) {
			ortFehler = /** @type {Error} */ (e).message;
		} finally {
			ortLaeuft = '';
		}
	}

	async function umziehen() {
		if (!ortZiel) return;
		ortFehler = '';
		ortLaeuft = 'umziehen';
		try {
			await post('/api/settings/location', { path: ortZiel.target });
			ortUnterwegs = ortZiel.target;
			ortZiel = null;
			await ladeStand();
		} catch (e) {
			ortFehler = /** @type {Error} */ (e).message;
		} finally {
			ortLaeuft = '';
		}
	}

	/** @param {string} key */
	async function oeffnen(key) {
		delete fehler[key];
		try {
			await post('/api/settings/open', { folder: key });
		} catch (e) {
			fehler[key] = /** @type {Error} */ (e).message;
		}
	}

	/** @param {string} name */
	async function herunterladen(name) {
		delete fehler[name];
		fordert.push(name);
		try {
			await post('/api/settings/download', { name });
		} catch (e) {
			fehler[name] = /** @type {Error} */ (e).message;
		}
		await ladeStand();
		fordert = fordert.filter((n) => n !== name);
	}

	// ---- GPU ----
	/** @type {Gpu | null} */
	let gpu = $state(null);
	let pruefe = $state(false);
	let gpuFehler = $state('');

	/** @param {boolean} [frisch] neu erkennen statt der gemerkten Antwort */
	async function ladeGpu(frisch = false) {
		try {
			gpu = await getJSON('/api/gpu' + (frisch ? '?fresh=1' : ''));
		} catch {
			// bleibt stehen bis zur nächsten Abfrage
		}
	}

	async function erneutPruefen() {
		pruefe = true;
		await ladeGpu(true);
		pruefe = false;
	}

	async function gpuLaden() {
		gpuFehler = '';
		try {
			await post('/api/gpu/enable');
			einrichtung.neu++;
		} catch (e) {
			gpuFehler = /** @type {Error} */ (e).message;
		}
		await Promise.all([ladeStand(), ladeGpu()]);
	}

	/** @param {boolean} an */
	async function mitGpu(an) {
		if (!gpu) return;
		gpu.accel = an ? 'cuda' : 'cpu';
		await speichern({ accel: gpu.accel });
		await ladeGpu();
	}

	// ---- Start und Hintergrund (nur Windows) ----
	/** ready: „Im Hintergrund bereit“ (fehlt ohne Live-Transkription)
	 * @type {{ supported: boolean, autostart: boolean, background: boolean, ready?: boolean } | null} */
	let hintergrund = $state(null);
	let hintergrundFehler = $state('');

	async function ladeHintergrund() {
		try {
			hintergrund = await getJSON('/api/background');
		} catch {
			// bleibt stehen bis zum nächsten Laden der Seite
		}
	}

	/** die Änderungen gehen nacheinander hinaus, in der Reihenfolge der Klicks */
	let hintergrundKette = Promise.resolve();
	/** nur die Antwort auf die letzte Änderung zählt */
	let hintergrundNr = 0;

	/** @param {{ autostart?: boolean, background?: boolean, ready?: boolean }} patch */
	function hintergrundSetzen(patch) {
		hintergrundFehler = '';
		// gleich übernehmen: ein zweiter Klick vor der Antwort schaltet vom neuen Stand aus
		if (hintergrund) hintergrund = { ...hintergrund, ...patch };
		const nr = ++hintergrundNr;
		hintergrundKette = hintergrundKette.then(async () => {
			try {
				const neu = await post('/api/background', patch);
				if (nr === hintergrundNr) hintergrund = neu;
			} catch (e) {
				hintergrundFehler = /** @type {Error} */ (e).message;
				await ladeHintergrund();
			}
		});
	}

	// ---- Diktieren per Tastenkürzel (nur Windows) ----
	/** @type {{ supported: boolean, key: string, insert: boolean, clipboard: boolean, keys: { key: string, label: string }[], error?: string } | null} */
	let diktat = $state(null);
	let diktatFehler = $state('');

	async function ladeDiktat() {
		try {
			diktat = await getJSON('/api/dictate');
		} catch {
			// bleibt stehen bis zum nächsten Laden der Seite
		}
	}

	/** @param {{ key?: string, insert?: boolean, clipboard?: boolean }} patch */
	async function diktatSetzen(patch) {
		diktatFehler = '';
		try {
			diktat = await post('/api/dictate', patch);
		} catch (e) {
			diktatFehler = /** @type {Error} */ (e).message;
			await ladeDiktat();
		}
	}

	// ---- Texterkennung (OCR) ----
	/** @typedef {{ id: string, name: string, version: string, ready: boolean, usable: boolean, languages: string[],
	 *   download: string[], layouts: string[], boxes: string, gpu: boolean, error?: string }} Leser */
	/** null: wird geladen; false: diese App kann keine Texterkennung
	 * @type {{ engines: Leser[] } | null | false} */
	let ocr = $state(null);
	/** die Sprachen zum Ausprobieren, mit + verbunden wie in der API */
	let ocrSprachen = $state('deu');
	let ocrLayout = $state('auto');
	/** die Erkennung zum Ausprobieren; leer: die erste, die liest */
	let ocrLeser = $state('');
	let liest = $state(false);
	/** @type {{ text: string, zeilen: number, sekunden: number, bild: string, leser: string, breite: number, hoehe: number,
	 *   kaesten: number[][] } | null} */
	let gelesen = $state(null);
	let ocrFehler = $state('');

	/** Tesseract's models in their order (models/status.go) */
	const OCR_REIHE = ['deu', 'eng', 'frk', 'Fraktur', 'frak2021', 'latin', 'fra', 'ita', 'spa', 'nld', 'pol', 'ces', 'dan', 'swe', 'nor', 'rus'];
	/** names of the models that are not a language; languages are named by Intl in the interface language */
	const OCR_SCHRIFTEN = /** @type {Record<string, string>} */ ({
		frk: /* @wc-include */ 'Fraktur (frk)',
		Fraktur: /* @wc-include */ 'Fraktur script',
		frak2021: /* @wc-include */ 'Fraktur (frak2021)',
		latin: /* @wc-include */ 'Latin script (German, English, …)'
	});
	const sprachName = new Intl.DisplayNames([getLocale()], { type: 'language' });
	/** was jede Erkennung gut kann (ocr) */
	const OCR_LESER = /** @type {Record<string, string>} */ ({
		tesseract: /* @wc-include */ 'Reads print and typewriting, with its own models for Fraktur.',
		onnx: /* @wc-include */ 'Reads printed Latin script very accurately, including index cards and lists, and knows where each word is; fast on the graphics card. For Fraktur, Tesseract is better.',
		os: /* @wc-include */ 'The text recognition built into Windows: available at once and very fast, with the position of each word, but without Fraktur or handwriting. Its languages come from Windows; you add more under Settings › Time & language › Language.',
		api: /* @wc-include */ 'An AI model reads the image, in any language and script, including handwriting and Kurrent. It finds the lines, but not the position of each word. For this, the image goes to the service, billed through your key.'
	});
	/** die Sprache, mit der eine Erkennung beim Ausprobieren anfängt
	 * @param {Leser} l */
	const ersteSprache = (l) => {
		const s = ocrReihe([...l.languages, ...l.download]);
		return s.includes('deu') ? 'deu' : (s[0] ?? 'deu');
	};
	const OCR_LAYOUTS = /** @type {Record<string, string>} */ ({
		auto: /* @wc-include */ 'Page with columns (automatic)',
		lines: /* @wc-include */ 'Lines, such as an index card or form',
		block: /* @wc-include */ 'A single block of text, such as a label',
		sparse: /* @wc-include */ 'Scattered, such as lettering on objects'
	});
	const ocrName = (/** @type {string} */ l) => OCR_SCHRIFTEN[l] ?? (OCR_REIHE.includes(l) ? sprachName.of(l) : undefined) ?? l;
	/** in der Reihenfolge oben, unbekannte danach */
	const ocrReihe = (/** @type {string[]} */ l) => {
		const rang = (/** @type {string} */ x) => {
			const i = OCR_REIHE.indexOf(x);
			return i < 0 ? 99 : i;
		};
		return l.toSorted((a, b) => rang(a) - rang(b));
	};

	/** die Erkennungen, die lesen können */
	const ocrLesbar = () => (ocr ? ocr.engines.filter((l) => l.usable) : []);

	async function ladeOcr() {
		try {
			const r = await fetch('/api/ocr/engines');
			// Listen ohne Einträge fehlen in der Antwort (alles installiert: kein download)
			ocr = r.ok
				? {
						engines: (await r.json()).engines.map((/** @type {Leser} */ l) => ({
							...l,
							languages: l.languages ?? [],
							download: l.download ?? [],
							layouts: l.layouts ?? []
						}))
					}
				: false;
		} catch {
			ocr = false;
		}
	}

	/** Liest das gewählte Bild und zeigt den Text.
	 * @param {Event & { currentTarget: HTMLInputElement }} e */
	async function bildLesen(e) {
		const f = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!f) return;
		ocrFehler = '';
		liest = true;
		if (gelesen) URL.revokeObjectURL(gelesen.bild);
		gelesen = null;
		const start = performance.now();
		try {
			const leser = ocrLesbar().find((l) => l.id === ocrLeser) ?? ocrLesbar()[0];
			const form = new FormData();
			form.append('image', f, f.name);
			if (leser) form.append('engine', leser.id);
			form.append('languages', ocrSprachen);
			form.append('layout', ocrLayout);
			const r = await fetch('/api/ocr', { method: 'POST', body: form });
			const j = await r.json().catch(() => ({}));
			if (!r.ok || j.ok === false) throw new Error(j.error || r.statusText);
			/** @typedef {{ text: string, para?: number, bbox?: number[], words?: { bbox?: number[] }[] }} Zeile */
			const seite = j.pages?.[0] ?? { width: 1, height: 1 };
			/** @type {Zeile[]} */
			const zeilen = (j.pages ?? []).flatMap((/** @type {{ lines: Zeile[] }} */ p) => p.lines);
			// Zeilen eines Absatzes zusammen, Absätze mit einer Leerzeile dazwischen
			const text = zeilen.map((z, i) => (i && z.para !== zeilen[i - 1].para ? '\n' : '') + z.text).join('\n');
			// die Kästen der Wörter, wo die Erkennung sie kennt, sonst die der Zeilen (nur die erste Seite)
			const kaesten = /** @type {number[][]} */ (
				(seite.lines ?? [])
					.flatMap((/** @type {Zeile} */ z) => (z.words?.some((w) => w.bbox) ? z.words.map((w) => w.bbox) : [z.bbox]))
					.filter(Boolean)
			);
			gelesen = {
				text,
				zeilen: zeilen.length,
				sekunden: (performance.now() - start) / 1000,
				bild: URL.createObjectURL(f),
				leser: leser?.name ?? '',
				breite: seite.width,
				hoehe: seite.height,
				kaesten
			};
			ladeOcr(); // die erste Lesung lädt Tesseract und Modelle herunter
		} catch (err) {
			ocrFehler = /** @type {Error} */ (err).message;
		} finally {
			liest = false;
		}
	}

	// ---- Texterkennung für Programme: Vorgabe und KI-Dienst (eigener Schlüssel, getrennt vom Dienst für die Sprache) ----
	/** @typedef {{ id: string, label: string, kind: string, url: string, model: string, keys?: string, noKey?: boolean }} KiDienst */
	/** @typedef {{ ocrEngine: string, ocrApi: Api, ocrApiProviders: KiDienst[] }} OcrDienst */
	/** @type {OcrDienst | null} */
	let ocrDienst = $state(null);
	let ocrSchluessel = $state('');
	let ocrSchluesselStand = $state('');
	let speichertOcrSchluessel = $state(false);
	let ocrDienstFehler = $state('');

	/** @param {OcrDienst} p */
	function ocrDienstAus(p) {
		const { ocrEngine, ocrApi, ocrApiProviders } = p;
		ocrDienst = { ocrEngine: ocrEngine ?? '', ocrApi, ocrApiProviders };
	}

	/** @param {Record<string, unknown>} patch */
	async function ocrDienstSetzen(patch) {
		ocrDienstFehler = '';
		try {
			ocrDienstAus(await post('/api/prefs', patch));
		} catch (e) {
			ocrDienstFehler = /* @wc-include */ `Not saved: ${/** @type {Error} */ (e).message}`;
			return false;
		}
		ladeOcr(); // ob der KI-Dienst liest, hängt an Schlüssel und Modell; die Vorgabe steht in der Liste vorn
		return true;
	}

	/** @param {Record<string, string>} ocrApi */
	async function ocrApiSetzen(ocrApi) {
		ocrSchluesselStand = '';
		await ocrDienstSetzen({ ocrApi });
	}

	/** @param {string} wert leer: den Schlüssel entfernen */
	async function ocrSchluesselSpeichern(wert) {
		speichertOcrSchluessel = true;
		ocrSchluesselStand = '';
		if (await ocrDienstSetzen({ ocrApi: { key: wert } })) {
			ocrSchluessel = '';
			ocrSchluesselStand = wert ? /* @wc-include */ 'Key saved.' : /* @wc-include */ 'Key removed.';
		}
		speichertOcrSchluessel = false;
	}

	// ---- Programme: die Adresse, unter der Programme Transcribe erreichen, und die der Live-API ----
	const rechner = typeof location === 'undefined' ? 'localhost:31765' : location.host;
	const grundadresse = `http://${rechner}`;
	const adresse = `ws://${rechner}/api/live`;
	let kopiert = $state(false);
	let kopierFehler = $state('');
	async function kopieren() {
		kopierFehler = '';
		try {
			await navigator.clipboard.writeText(grundadresse);
			kopiert = true;
			setTimeout(() => (kopiert = false), 2000);
		} catch {
			kopierFehler = /* @wc-include */ 'Copying didn’t work.';
		}
	}

	// ---- Menü: Kategorien und ihre Abschnitte ----
	const KATEGORIEN = /** @type {const} */ ([
		{
			id: 'transkribieren',
			name: /* @wc-include */ 'Transcribing',
			abschnitte: [
				[
					'erkennung',
					/* @wc-include */ 'Recognition'
				],
				[
					'neu',
					/* @wc-include */ 'New recordings'
				],
				[
					'ordner',
					/* @wc-include */ 'Connected folders'
				]
			]
		},
		{
			id: 'live',
			name: /* @wc-include */ 'Live and dictation',
			abschnitte: [
				[
					'aufnahme',
					/* @wc-include */ 'Recording'
				],
				[
					'stimme',
					/* @wc-include */ 'My voice'
				],
				[
					'diktieren',
					/* @wc-include */ 'Dictation'
				]
			]
		},
		{
			id: 'programme',
			name: /* @wc-include */ 'Apps',
			abschnitte: [
				[
					'zugriff',
					/* @wc-include */ 'Access'
				],
				[
					'bereit',
					/* @wc-include */ 'In the background'
				],
				[
					'adresse',
					/* @wc-include */ 'Address'
				],
				[
					'anfragen',
					/* @wc-include */ 'Saved requests'
				]
			]
		},
		{
			id: 'ocr',
			name: /* @wc-include */ 'Text recognition',
			abschnitte: [
				[
					'ocr-leser',
					/* @wc-include */ 'Engines'
				],
				[
					'ocr-ki',
					/* @wc-include */ 'AI service'
				],
				[
					'ocr-probe',
					/* @wc-include */ 'Try it'
				]
			]
		},
		{
			id: 'system',
			name: /* @wc-include */ 'System',
			abschnitte: [
				[
					'sprache',
					/* @wc-include */ 'Language'
				],
				[
					'gpu',
					/* @wc-include */ 'Graphics card'
				],
				[
					'speicher',
					/* @wc-include */ 'Storage and models'
				],
				[
					'programm',
					/* @wc-include */ 'About Transcribe'
				]
			]
		}
	]);
	/** was diese App zeigt: ohne Texterkennung keine Kategorie dafür, ohne Tastenkürzel kein Diktieren */
	const menue = $derived(
		KATEGORIEN.filter((k) => k.id !== 'ocr' || ocr !== false).map((k) => ({
			...k,
			abschnitte: k.abschnitte.filter(([id]) => id !== 'diktieren' || !!diktat?.supported)
		}))
	);
	/** die Kategorie, die gerade zu sehen ist */
	let kategorie = $state('transkribieren');
	const offen = $derived(menue.find((k) => k.id === kategorie) ?? menue[0]);
	let aktuell = $state('erkennung');
	/** im Menü angeklickt: bleibt markiert, bis man selbst scrollt, auch wenn der Sprung ganz unten endet */
	let angeklickt = '';
	/** @type {HTMLElement | undefined} */
	let menueLeiste = $state();

	/** die Kategorie, zu der ein #anker gehört: sie selbst oder einer ihrer Abschnitte
	 * @param {string} anker ohne # */
	const kategorieVon = (anker) => KATEGORIEN.find((k) => k.id === anker || k.abschnitte.some(([id]) => id === anker));

	/** Öffnet, was der #anker nennt: eine Kategorie (von oben) oder einen Abschnitt (in seiner Kategorie).
	 * @param {string} anker ohne # */
	async function zeigen(anker) {
		const k = kategorieVon(anker);
		if (!k) return;
		const abschnitt = k.id === anker ? '' : anker;
		kategorie = k.id;
		aktuell = angeklickt = abschnitt || k.abschnitte[0][0];
		await tick();
		if (abschnitt) document.getElementById(abschnitt)?.scrollIntoView();
		else scrollTo({ top: 0 });
	}

	// schmal scrollt die Leiste seitlich: die offene Kategorie bleibt darin zu sehen
	$effect(() => {
		const a = menueLeiste?.querySelector(`a[href="#${offen.id}"]`);
		if (!menueLeiste || !(a instanceof HTMLElement) || menueLeiste.scrollWidth <= menueLeiste.clientWidth) return;
		const leiste = menueLeiste.getBoundingClientRect();
		const r = a.getBoundingClientRect();
		if (r.left < leiste.left || r.right > leiste.right)
			menueLeiste.scrollBy({ left: r.left - leiste.left - 16, behavior: 'smooth' });
	});

	/** Merkt sich, welcher Abschnitt oben im Fenster steht.
	 * @param {HTMLElement} el */
	function beobachten(el) {
		const sichtbar = new Map();
		const io = new IntersectionObserver(
			(eintraege) => {
				for (const e of eintraege) sichtbar.set(e.target.id, e.isIntersecting);
				if (angeklickt) return;
				// ganz unten zählt der letzte Abschnitt, auch wenn er kurz ist
				const unten = innerHeight + scrollY >= document.documentElement.scrollHeight - 4;
				const ids = offen.abschnitte.map(([id]) => id);
				aktuell = unten ? ids[ids.length - 1] : (ids.find((id) => sichtbar.get(id)) ?? aktuell);
			},
			{ rootMargin: '-140px 0px -50% 0px' }
		);
		const alle = () => el.querySelectorAll('section[id]').forEach((s) => io.observe(s));
		alle();
		const mo = new MutationObserver(alle);
		mo.observe(el, { childList: true });
		// wer selbst scrollt, sieht wieder den Abschnitt oben im Fenster
		const selbst = () => (angeklickt = '');
		const arten = ['wheel', 'touchmove', 'keydown', 'pointerdown'];
		for (const a of arten) addEventListener(a, selbst, { passive: true, capture: true });
		return () => {
			io.disconnect();
			mo.disconnect();
			for (const a of arten) removeEventListener(a, selbst, { capture: true });
		};
	}

	// ---- Version und Update ----
	/** die Version vor einem Update, das gerade neu geladen hat; '' sonst */
	let vorher = $state('');
	/** 'check' oder 'install', solange die App antwortet */
	let updateLaeuft = $state('');
	let updateFehler = $state('');

	/** @param {'check' | 'install'} action */
	async function updateTun(action) {
		updateLaeuft = action;
		updateFehler = '';
		try {
			await updateAktion(action);
		} catch (e) {
			updateFehler = /** @type {Error} */ (e).message;
		}
		updateLaeuft = '';
	}

	/** @param {number} sekunden */
	function wann(sekunden) {
		const d = new Date(sekunden * 1000);
		const heute = new Date().toDateString() === d.toDateString();
		const uhr = d.toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' });
		return heute ? /* @wc-include */ `today at ${uhr}` : /* @wc-include */ `on ${d.toLocaleDateString(getLocale())} at ${uhr}`;
	}

	onMount(() => {
		vorher = vorherigeVersion();
		const geladen = Promise.allSettled([
			ladeUpdate(),
			ladeOcr(),
			ladeHintergrund(),
			ladeDiktat(),
			ladeVorgaben().catch(() => {
				gespeichert = /* @wc-include */ 'The defaults could not be loaded.';
				gespeichertFehler = true;
			}),
			ladeVerbunden().catch(() => {}),
			ladeStand(),
			ladeGpu()
		]);
		// mit #kategorie oder #abschnitt geöffnet (das Update in der App-Leiste, das Menü im Infobereich,
		// „Programme verbinden“ auf der Seite Live): die Kategorie gleich, den Sprung zum Abschnitt erst,
		// wenn die Abschnitte darüber geladen sind, sonst landet er zu weit oben
		const anker = () => decodeURIComponent(location.hash.slice(1));
		if (anker()) {
			kategorie = kategorieVon(anker())?.id ?? kategorie;
			geladen.then(() => zeigen(anker()));
		}
		const wechsel = () => zeigen(anker());
		addEventListener('hashchange', wechsel);
		const liveWeg = live.verbinden();
		const s = setInterval(einzeln(ladeStand), 2000);
		const g = setInterval(einzeln(() => ladeGpu()), 4000);
		return () => {
			clearInterval(s);
			clearInterval(g);
			clearTimeout(gespeichertUhr);
			removeEventListener('hashchange', wechsel);
			liveWeg();
		};
	});
</script>

<svelte:head><title>Settings · Transcribe by Moliri</title></svelte:head>

<main>
	<header class="kopf">
		<h1>Settings</h1>
		<p>Changes apply at once; there is nothing to save.</p>
	</header>

	<div class="rahmen">
		<nav class="menue" aria-label="Settings categories" bind:this={menueLeiste}>
			<ul>
				{#each menue as k (k.id)}
					<li>
						<a class="kategorie" href="#{k.id}" aria-current={offen.id === k.id ? 'page' : undefined}>{k.name}</a>
						{#if offen.id === k.id}
							<ul class="unter" aria-label="Sections">
								{#each k.abschnitte as [id, name] (id)}
									<li>
										<a href="#{id}" aria-current={aktuell === id ? 'location' : undefined} onclick={() => (aktuell = angeklickt = id)}>{name}</a>
									</li>
								{/each}
							</ul>
						{/if}
					</li>
				{/each}
			</ul>
		</nav>

		<div class="abschnitte" {@attach beobachten}>
			{#if offen.id === 'transkribieren'}
			<section id="erkennung" class="blatt" aria-labelledby="h-erkennung">
				<h2 id="h-erkennung">Recognition</h2>
				<p class="hinweis">
					Speech in your recordings is recognized by the models on this computer or by a service through its API, on the internet
					or on your own network.
				</p>
				{#if !erkennung}
					<p class="leer">Loading…</p>
				{:else}
					{@const e = erkennung}
					<fieldset class="arten">
						<legend class="sr">Where speech is recognized</legend>
						{#each ARTEN as [wert, titel, text] (wert)}
							<label class="art" class:gewaehlt={e.recognition === wert}>
								<input
									type="radio"
									name="erkennung"
									value={wert}
									checked={e.recognition === wert}
									onchange={() => erkennungSetzen({ recognition: wert })}
								/>
								<span>
									<strong>{titel}</strong>
									<span class="meta">{text}</span>
								</span>
							</label>
						{/each}
					</fieldset>

					{#if e.recognition !== 'local'}
						{@const a = e.api}
						{@const dienst = e.apiProviders.find((d) => d.id === a.provider) ?? e.apiProviders[0]}
						<div class="api">
							<div class="apifelder">
								<label class="feldname">
									Service
									<select class="feld" value={a.provider} onchange={(ev) => apiSetzen({ provider: ev.currentTarget.value })}>
										{#each e.apiProviders as d (d.id)}<option value={d.id}>{d.label}</option>{/each}
									</select>
								</label>
								<label class="feldname adresse">
									Address
									<input
										class="feld"
										spellcheck="false"
										value={a.url}
										onchange={(ev) => apiSetzen({ url: ev.currentTarget.value })}
									/>
								</label>
								<label class="feldname">
									Model
									<input class="feld" spellcheck="false" value={a.model} onchange={(ev) => apiSetzen({ model: ev.currentTarget.value })} />
								</label>
							</div>
							<p class="hinweis">
								{#if dienst.kind === 'openai'}
									Speaks the OpenAI-compatible interface, like OpenAI, Groq, Mistral or a server of your own (such as Speaches or LocalAI).
									It does not tell speakers apart, and Live only shows the text after each pause.
								{:else}
									Tells speakers apart and recognizes sounds such as laughter or applause; Live shows the text while you speak.
								{/if}
							</p>

							<label class="feldname" for="schluessel"
								>{dienst.noKey ? /* @wc-include */ 'API key (if the server requires one)' : /* @wc-include */ 'API key'}</label
							>
							<form
								class="schluesselzeile"
								onsubmit={(ev) => {
									ev.preventDefault();
									if (schluessel.trim()) schluesselSpeichern(schluessel.trim());
								}}
							>
								<input
									id="schluessel"
									class="feld"
									type="password"
									autocomplete="off"
									spellcheck="false"
									placeholder={a.keySet
										? a.keyHint
											? /* @wc-include */ `stored (ends in ${a.keyHint})`
											: /* @wc-include */ 'stored'
										: /* @wc-include */ 'Key'}
									bind:value={schluessel}
								/>
								<Button type="primary" size="large" nativeType="submit" disabled={!schluessel.trim()} loading={speichertSchluessel}>
									{a.keySet ? /* @wc-include */ 'Replace' : /* @wc-include */ 'Save'}
								</Button>
								{#if a.keySet}
									<Button type="secondary" size="large" onclick={() => schluesselSpeichern('')}>Remove</Button>
								{/if}
							</form>
							{#if schluesselStand}
								<p class="gespeichert" role="status">{schluesselStand}</p>
							{:else if !a.keySet && !dienst.noKey}
								<p class="warnung">Without a key, the app can’t use {dienst.label}.</p>
							{/if}
							<p class="hinweis">
								{#if dienst.keys}
									You create the key in your account at
									<a href={dienst.keys} target="_blank" rel="noreferrer">{dienst.label}</a>. Billing is usually per minute of audio.
								{/if}
								The key stays in the settings on this computer.
							</p>
						</div>
					{/if}

					{#if e.recognition === 'both'}
						<div class="schalter">
							<label class="feldname">
								Live and dictation
								<select class="feld" value={e.liveEngine} onchange={(ev) => erkennungSetzen({ liveEngine: ev.currentTarget.value })}>
									{#each Object.entries(ENGINES) as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
								</select>
							</label>
							<label class="feldname">
								Whole recordings
								<select class="feld" value={e.fileEngine} onchange={(ev) => erkennungSetzen({ fileEngine: ev.currentTarget.value })}>
									{#each Object.entries(ENGINES) as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
								</select>
							</label>
						</div>
						<p class="hinweis">
							For whole recordings this is the default: in the library and when you drop files, you choose for each recording what
							should recognize it.
						</p>
					{/if}
					{#if erkennungFehler}<p class="warnung">{erkennungFehler}</p>{/if}
				{/if}
			</section>

			<section id="neu" class="blatt" aria-labelledby="h-neu">
				<div class="zeile">
					<h2 id="h-neu">New recordings</h2>
					<p class="gespeichert" class:fehler={gespeichert && gespeichertFehler} role="status">{gespeichert}</p>
				</div>
				<p class="hinweis">
					Every recording you drag into the window or transcribe in the library starts with these. You can change them there for
					individual recordings.
				</p>

				<!-- gesperrt, bis die Vorgaben da sind: eine Änderung davor ginge verloren -->
				<fieldset class="vorgaben" disabled={!geladen}>
					<AuftragsOptionen bind:optionen onchange={optionenSpeichern} erkennung={erkennung?.recognition} />
				</fieldset>

				<dl class="erklaerung">
					<dt>Speakers</dt>
					<dd>
						Usually the app detects by itself how many people are speaking. A fixed number helps when voices sound alike or someone
						speaks only briefly.
					</dd>
					<dt>Languages</dt>
					<dd>
						On automatic, the app recognizes any language. If you name the languages that occur, it chooses only among them; this
						helps with dialect and short passages in other languages.
					</dd>
				</dl>

				<div class="ablegen">
					<h3 class="feldname">When you drag files or folders into the window</h3>
					<Toggle
						bind:checked={sofort}
						disabled={!geladen}
						title="Start right away with these defaults"
						description={sofort
							? /* @wc-include */ 'The recordings are transcribed right away.'
							: /* @wc-include */ 'Off: the app first asks about speakers and languages and starts when you confirm.'}
						onclick={() => speichern({ dropMode: sofort ? 'auto' : 'ask' })}
					/>
				</div>
			</section>

			<section id="ordner" class="blatt" aria-labelledby="h-ordner">
				<div class="zeile">
					<h2 id="h-ordner">Connected folders</h2>
					<!-- nur da, wenn es etwas sagt: die Seite hat sonst einen Status, den der übrigen Vorgaben -->
					{#if ordnerStand}<p class="gespeichert" class:fehler={ordnerFehler} role="status">{ordnerStand}</p>{/if}
				</div>
				<p class="hinweis">
					The recordings in a connected folder appear in the library without being copied, including those in its subfolders.
					You can connect a folder in the <a href="/">library</a>. Transcribe never deletes anything in it.
				</p>
				{#if verbunden?.folders.length}
					<ul class="liste" aria-label="Connected folders">
						{#each verbunden.folders as f (f.path)}
							<li class="eintrag">
								<div class="was">
									<span class="name">{f.name}</span>
									<span class="pfad">{f.exists ? f.path : /* @wc-include */ `${f.path} (not reachable)`}</span>
								</div>
								<span></span>
								<div class="tun">
									<Button type="secondary" size="large" aria-label="Disconnect {f.name}" onclick={() => verbundenAendern({ action: 'remove', path: f.path })}>
										{#snippet iconLeft()}<IconMaterialSymbolsLinkOffRounded />{/snippet}
										Disconnect
									</Button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
				<div class="ablegen">
					<Toggle
						bind:checked={nebenAn}
						disabled={!verbunden}
						title="Save transcripts next to the recordings"
						description={nebenAn
							? /* @wc-include */ 'Every transcript is also saved as a file next to its recording (“Interview.mp3.transcribe.json”), with corrections and clips. Anyone you share the folder with, for example via Nextcloud, sees and edits the same transcripts.'
							: /* @wc-include */ 'Off: transcripts stay in Transcribe only. If there is already one next to a recording, the app reads it anyway.'}
						onclick={() => verbundenAendern({ action: 'sidecar', on: nebenAn })}
					/>
				</div>
			</section>
			{/if}


			{#if offen.id === 'programme'}
				<section id="zugriff" class="blatt" aria-labelledby="h-zugriff">
					<h2 id="h-zugriff">Access</h2>
					<p class="hinweis">
						Apps like Moliri get speech as text from Transcribe and have it read text from images. Apps on this computer may
						always do so, websites from other addresses only once you allow them. When one tries, Transcribe asks wherever you
						are, and with a notification if the window is closed.
					</p>
					{#if live.abgelehnt.length}
						<div class="fragen">
							<h3 class="teil">Want to use Transcribe</h3>
							<ul class="liste" aria-label="Want to use Transcribe">
								{#each live.abgelehnt as z (z.origin)}
									<li class="eintrag">
										<div class="was">
											<span class="name">{z.origin === 'null' ? /* @wc-include */ 'A file or embedded page' : z.origin}</span>
											<span class="pfad">
												{z.client && !z.origin.endsWith(`//${z.client}`) && z.client !== z.origin ? `${z.client}, ` : ''}asked at {uhrzeit(z.at)}
											</span>
										</div>
										<span></span>
										<div class="tun">
											<Button type="primary" size="large" onclick={() => live.freigeben(z.origin, true)}>Allow</Button>
											<Button type="secondary" size="large" onclick={() => live.freigeben(z.origin, false)}>Deny</Button>
										</div>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
					<h3 class="teil">Allowed websites</h3>
					{#if live.erlaubt.length}
						<ul class="liste" aria-label="Allowed websites">
							{#each live.erlaubt as o (o)}
								<li class="eintrag">
									<div class="was"><span class="name">{o}</span></div>
									<span></span>
									<div class="tun">
										<Button type="tertiary" size="large" onclick={() => live.freigeben(o, false)}>Stop allowing</Button>
									</div>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="leer">No website allowed yet.</p>
					{/if}
					<p class="meta">The permission applies to live transcription and text recognition.</p>
				</section>

				<section id="bereit" class="blatt" aria-labelledby="h-bereit">
					<h2 id="h-bereit">In the background</h2>
					<p class="hinweis">
						Apps can only reach Transcribe while it is running; if you like, also without an open window, as an icon in the
						notification area of the taskbar.
					</p>
					{#if hintergrund}
						{@const h = hintergrund}
						<div class="schalter">
							{#if h.ready !== undefined}
								<Toggle
									checked={h.ready}
									title="Ready in the background"
									description={h.ready
										? /* @wc-include */ 'Apps may have Transcribe record from the microphone, and recognition stays loaded so that everything starts at once. This takes about 1 GB of memory. Transcribe always processes audio and images that an app sends.'
										: /* @wc-include */ 'Off: apps may not have Transcribe record from the microphone, and nothing stays loaded. Transcribe still processes audio and images that an app sends. Recording and dictating by hand work too; they just start a little more slowly.'}
									onclick={() => hintergrundSetzen({ ready: !h.ready })}
								/>
							{/if}
							{#if h.supported}
								<Toggle
									checked={h.autostart}
									title="Start with Windows"
									description={h.autostart
										? /* @wc-include */ 'Transcribe starts without a window when you sign in and waits in the notification area.'
										: /* @wc-include */ 'Off: Transcribe only starts when you open it.'}
									onclick={() => hintergrundSetzen({ autostart: !h.autostart })}
								/>
								<Toggle
									checked={h.background || h.autostart}
									disabled={h.autostart}
									title="Keep running after closing"
									description={h.autostart
										? /* @wc-include */ 'Always on while Transcribe starts with Windows.'
										: h.background
											? /* @wc-include */ 'When you close the window, Transcribe stays in the notification area: click it to open it, right-click it to quit.'
											: /* @wc-include */ 'Off: when you close the window, Transcribe quits as soon as there is nothing left to do.'}
									onclick={() => hintergrundSetzen({ background: !h.background })}
								/>
							{/if}
						</div>
						{#if hintergrundFehler}<p class="warnung">{hintergrundFehler}</p>{/if}
					{/if}
				</section>

				<section id="adresse" class="blatt" aria-labelledby="h-adresse">
					<h2 id="h-adresse">Address</h2>
					<p class="hinweis">Apps reach Transcribe at this address. In Moliri, you enter it in your profile:</p>
					<div class="apiadresse">
						<code>{grundadresse}</code>
						<Button type="secondary" size="large" onclick={kopieren}>
							{#snippet iconLeft()}<IconMaterialSymbolsContentCopyOutlineRounded />{/snippet}
							{kopiert ? /* @wc-include */ 'Copied' : /* @wc-include */ 'Copy address'}
						</Button>
					</div>
					{#if kopierFehler}<p class="warnung" role="status">{kopierFehler}</p>{/if}
					<dl class="modi">
						<dt>Send audio</dt>
						<dd>The app records by itself and sends the sound; the text comes back right away.</dd>
						<dt>Let Transcribe listen in <code>?mode=mic</code></dt>
						<dd>
							The app only says “from now on” and “up to now”. In between, Transcribe records what is selected under
							<a href="#aufnahme">Live and dictation › Recording</a> and keeps sending the text to the app.
							{#if !live.an}<strong>This only works while “Ready in the background” is on.</strong>{/if}
						</dd>
					</dl>
					<p class="meta">
						Every request appears on the <a href="/live">Live</a> page as a recording “Via API”; what apps have read, ask the
						language model or convert for search by meaning appears there under “AI & OCR”. As long as Transcribe is delivering
						text to an app, it does not quit.
					</p>
					<details class="protokoll">
						<summary>For developers: how to talk to Transcribe</summary>
						<p>
							WebSocket at <code>{adresse}</code>. Parameters: <code>mode=audio</code> (default) or <code>mic</code>,
							<code>name=</code> what the recording should be called, for audio <code>sample_rate=16000</code> and
							<code>encoding=s16le</code> or <code>f32le</code> (mono).
						</p>
						<p>
							Audio goes in as binary messages. JSON messages come back: <code>ready</code>, continuously
							<code>partial</code> (the sentence being spoken), <code>final</code> (finished sentence, with
							<code>start</code> and <code>end</code> in seconds), at the end <code>done</code>, on problems
							<code>error</code>. <code>{'{"type":"stop"}'}</code> ends it: Transcribe still sends what has been said so
							far, then <code>done</code>.
						</p>
						<pre>{`const ws = new WebSocket('${adresse}?mode=mic&name=Moliri');
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.type === 'partial') feld.placeholder = m.text;
  if (m.type === 'final') feld.value += m.text + ' ';
};
// later: ws.send(JSON.stringify({ type: 'stop' }));`}</pre>
					</details>
				</section>

				<section id="anfragen" class="blatt" aria-labelledby="h-anfragen">
					<h2 id="h-anfragen">Saved requests</h2>
					<div class="schalter">
						<Toggle
							bind:checked={() => live.apiSpeichern, (v) => live.apiSpeichernSetzen(v)}
							title="Save requests as recordings"
							description={live.apiSpeichern
								? /* @wc-include */ 'Every request stays on the Live page, with its text, even after a restart.'
								: /* @wc-include */ 'Off: a request only appears on the Live page while it runs and for a few minutes afterwards. Nothing of it stays on the computer.'}
						/>
						{#if live.apiSpeichern}
							<label class="feldname">
								Delete automatically
								<select class="feld" value={live.apiLoeschen} onchange={(e) => live.apiLoeschenSetzen(Number(e.currentTarget.value))}>
									{#each LOESCHEN as [stunden, name] (stunden)}<option value={stunden}>{name}</option>{/each}
									{#if !LOESCHEN.some(([stunden]) => stunden === live.apiLoeschen)}
										<option value={live.apiLoeschen}>After {live.apiLoeschen} hours</option>
									{/if}
								</select>
							</label>
							<p class="meta">
								Counted from the end of a request; this also applies to those already saved. Recordings you start on the Live page
								and dictations are kept.
							</p>
						{/if}
					</div>
					{#if live.meldung}<p class="warnung" role="status">{live.meldung}</p>{/if}
				</section>
			{/if}

			{#if offen.id === 'ocr' && ocr !== false}
				<section id="ocr-leser" class="blatt" aria-labelledby="h-ocr">
					<h2 id="h-ocr">Text recognition</h2>
					<p class="hinweis">
						Transcribe also reads text from images, such as index cards, labels or Fraktur print, for apps like Moliri. The image
						does not leave the computer unless you let an AI service read it.
					</p>
					{#if !ocr}
						<p class="leer">Loading…</p>
					{:else}
						{#each ocr.engines as l (l.id)}
							<div class="leser">
								<p class="karte">
									<IconMaterialSymbolsDocumentScannerOutlineRounded aria-hidden="true" />
									<strong>{l.name}</strong>
									<span class="meta">
										{l.id === 'api'
											? /* @wc-include */ `Model ${l.version}`
											: l.version
												? /* @wc-include */ `Version ${l.version}`
												: l.gpu
													? /* @wc-include */ 'on the graphics card'
													: ''}
									</span>
									{#if l.ready}
										<span class="zustand da"><IconMaterialSymbolsCheckRounded aria-hidden="true" />ready</span>
									{:else if l.usable}
										<span class="zustand">downloads on first read</span>
									{:else}
										<span class="zustand fehlt">{l.id === 'api' ? /* @wc-include */ 'not set up' : /* @wc-include */ 'not installed'}</span>
									{/if}
								</p>
								<p class="hinweis">
									{OCR_LESER[l.id] ?? ''}
									{#if l.languages.length && l.id !== 'api'}Installed: {ocrReihe(l.languages).map(ocrName).join(', ')}.{/if}
									{#if l.download.length}
										Downloaded on first read: {ocrReihe(l.download).map(ocrName).join(', ')}.{/if}
									{#if !l.usable && l.id === 'tesseract'}
										On Linux and macOS, the installed program <code>tesseract</code> does the reading; install it with its languages,
										then reload this page.
									{:else if !l.usable && l.error}
										{l.error}.
									{/if}
								</p>
							</div>
						{/each}

						{#if ocrDienst}
							{@const od = ocrDienst}
							{@const vorgabe = ocr.engines.find((l) => l.id === od.ocrEngine)}
							<div class="schalter">
								<label class="feldname">
									For Moliri and other apps
									<select class="feld" value={od.ocrEngine} onchange={(ev) => ocrDienstSetzen({ ocrEngine: ev.currentTarget.value })}>
										<option value="">{/* @wc-include */ 'Automatic'}</option>
										{#each ocr.engines as l (l.id)}<option value={l.id}>{l.name}</option>{/each}
									</select>
								</label>
							</div>
							<p class="hinweis">
								Transcribe reads with this when an app names no engine, as Moliri does. Automatic takes the first one that can read,
								never the AI service.
							</p>
							{#if vorgabe && !vorgabe.usable}
								<p class="warnung">{vorgabe.name} can’t read yet; until then, Transcribe chooses automatically.</p>
							{/if}

							{#if ocrDienstFehler}<p class="warnung">{ocrDienstFehler}</p>{/if}
						{/if}
					{/if}
				</section>

				{#if ocr && ocrDienst}
					{@const od = ocrDienst}
					{@const a = od.ocrApi}
					{@const dienst = od.ocrApiProviders.find((d) => d.id === a.provider) ?? od.ocrApiProviders[0]}
					<section id="ocr-ki" class="blatt" aria-labelledby="h-ocr-ki">
						<h2 id="h-ocr-ki">AI service</h2>
						<p class="hinweis">
							With your own key, an AI model on the internet or on your own network does the reading. This is a different service
							from the one for speech under <a href="#erkennung">Transcribing › Recognition</a>, with its own model and key.
						</p>
						<div class="api">
							<div class="apifelder">
								<label class="feldname">
									Service
									<select class="feld" value={a.provider} onchange={(ev) => ocrApiSetzen({ provider: ev.currentTarget.value })}>
										{#each od.ocrApiProviders as d (d.id)}<option value={d.id}>{d.label}</option>{/each}
									</select>
								</label>
								<label class="feldname adresse">
									Address
									<input class="feld" spellcheck="false" value={a.url} onchange={(ev) => ocrApiSetzen({ url: ev.currentTarget.value })} />
								</label>
								<label class="feldname">
									Model
									<input class="feld" spellcheck="false" value={a.model} onchange={(ev) => ocrApiSetzen({ model: ev.currentTarget.value })} />
								</label>
							</div>
							{#if dienst.kind === 'openai'}
								<p class="hinweis">
									Speaks the OpenAI-compatible interface, like OpenAI, Mistral or a server of your own (such as Ollama). The model
									must be able to read images, such as qwen2.5vl.
								</p>
							{/if}

							<label class="feldname" for="ocr-schluessel"
								>{dienst.noKey ? /* @wc-include */ 'API key (if the server requires one)' : /* @wc-include */ 'API key'}</label
							>
							<form
								class="schluesselzeile"
								onsubmit={(ev) => {
									ev.preventDefault();
									if (ocrSchluessel.trim()) ocrSchluesselSpeichern(ocrSchluessel.trim());
								}}
							>
								<input
									id="ocr-schluessel"
									class="feld"
									type="password"
									autocomplete="off"
									spellcheck="false"
									placeholder={a.keySet
										? a.keyHint
											? /* @wc-include */ `stored (ends in ${a.keyHint})`
											: /* @wc-include */ 'stored'
										: /* @wc-include */ 'Key'}
									bind:value={ocrSchluessel}
								/>
								<Button type="primary" size="large" nativeType="submit" disabled={!ocrSchluessel.trim()} loading={speichertOcrSchluessel}>
									{a.keySet ? /* @wc-include */ 'Replace' : /* @wc-include */ 'Save'}
								</Button>
								{#if a.keySet}
									<Button type="secondary" size="large" onclick={() => ocrSchluesselSpeichern('')}>Remove</Button>
								{/if}
							</form>
							{#if ocrSchluesselStand}
								<p class="gespeichert" role="status">{ocrSchluesselStand}</p>
							{:else if !a.keySet && !dienst.noKey}
								<p class="warnung">Without a key, the app can’t use {dienst.label}.</p>
							{/if}
							<p class="hinweis">
								{#if dienst.keys}
									You create the key in your account at
									<a href={dienst.keys} target="_blank" rel="noreferrer">{dienst.label}</a>. Billing is per token, for each image and for the text the model writes.
								{/if}
								The key stays in the settings on this computer; Moliri does not get it.
							</p>
						</div>
						{#if ocrDienstFehler}<p class="warnung">{ocrDienstFehler}</p>{/if}
					</section>
				{/if}

				{#if ocr}
					{@const lesbar = ocrLesbar()}
					{@const leser = lesbar.find((l) => l.id === ocrLeser) ?? lesbar[0]}
					<section id="ocr-probe" class="blatt" aria-labelledby="h-ocr-probe">
						<h2 id="h-ocr-probe">Try it</h2>
						<p class="hinweis">Read an image of your own, with the engine, language and kind of text you choose.</p>
						{#if !leser}
							<p class="leer">No engine can read yet.</p>
						{:else}
							{@const sprachen = ocrReihe([...leser.languages, ...leser.download])}
							<div class="probe">
								<div class="apifelder">
									{#if lesbar.length > 1}
										<label class="feldname">
											Engine
											<select
												class="feld"
												value={leser.id}
												onchange={(e) => {
													ocrLeser = e.currentTarget.value;
													const l = lesbar.find((x) => x.id === ocrLeser);
													if (l) {
														ocrSprachen = ersteSprache(l);
														if (!l.layouts.includes(ocrLayout)) ocrLayout = l.layouts[0] ?? 'auto';
													}
												}}
											>
												{#each lesbar as l (l.id)}<option value={l.id}>{l.name}</option>{/each}
											</select>
										</label>
									{/if}
									<label class="feldname">
										Language and script
										<select class="feld" bind:value={ocrSprachen}>
											{#each sprachen as s (s)}<option value={s}>{ocrName(s)}</option>{/each}
											{#if sprachen.includes('deu') && sprachen.includes('frak2021')}
												<option value="deu+frak2021">German and Fraktur</option>
											{/if}
											{#if sprachen.includes('deu') && sprachen.includes('eng')}
												<option value="deu+eng">German and English</option>
											{/if}
										</select>
									</label>
									{#if leser.layouts.length > 1}
										<label class="feldname">
											What the image shows
											<select class="feld" bind:value={ocrLayout}>
												{#each leser.layouts as la (la)}<option value={la}>{OCR_LAYOUTS[la] ?? la}</option>{/each}
											</select>
										</label>
									{/if}
								</div>
								<label class="bildwahl" class:liest>
									{#if liest}<IconEosIconsLoading aria-hidden="true" />{:else}<IconMaterialSymbolsImageOutlineRounded aria-hidden="true" />{/if}
									{liest ? /* @wc-include */ 'Reading…' : /* @wc-include */ 'Read image'}
									<input class="sr" type="file" accept="image/*" disabled={liest} onchange={bildLesen} />
								</label>
								{#if liest && leser.id === 'api'}
									<p class="hinweis">The image is with the AI service. Large models think about a full page for a while.</p>
								{:else if liest && !leser.ready}
									<p class="hinweis">
										{leser.id === 'onnx'
											? /* @wc-include */ 'The first time, Transcribe downloads the PP-OCRv5 models (13 MB); this takes a moment.'
											: /* @wc-include */ 'The first time, Transcribe downloads Tesseract and the language; this takes a moment.'}
									</p>
								{/if}
								{#if ocrFehler}<p class="warnung">{ocrFehler}</p>{/if}
								{#if gelesen}
									<div class="ergebnis">
										<svg
											class="seite"
											viewBox="0 0 {gelesen.breite} {gelesen.hoehe}"
											role="img"
											aria-label="The image that was read, with the words found outlined"
										>
											<image href={gelesen.bild} width={gelesen.breite} height={gelesen.hoehe} />
											{#each gelesen.kaesten as k, i (i)}<rect x={k[0]} y={k[1]} width={k[2]} height={k[3]} />{/each}
										</svg>
										<div class="gelesen">
											<p class="meta">
												{plural(gelesen.zeilen, ['# line', '# lines'])} in {gelesen.sekunden.toLocaleString(getLocale(), {
													minimumFractionDigits: 1,
													maximumFractionDigits: 1
												})} s{gelesen.leser ? `, ${gelesen.leser}` : ''}
											</p>
											<pre>{gelesen.text || /* @wc-include */ 'No text found.'}</pre>
										</div>
									</div>
								{/if}
							</div>
						{/if}
					</section>
				{/if}
			{/if}

			{#if offen.id === 'live'}
				<section id="aufnahme" class="blatt" aria-labelledby="h-aufnahme">
					<h2 id="h-aufnahme">Recording</h2>
					<p class="hinweis">
						This is how Transcribe records on the <a href="/live">Live</a> page, when dictating and for apps that let it listen
						in. You can also reach source, microphone and languages there under “Set up”.
					</p>
					<div class="schalter"><AufnahmeWahl /></div>
					{#if live.meldung}<p class="hinweis" role="status">{live.meldung}</p>{/if}
				</section>

				<section id="stimme" class="blatt" aria-labelledby="h-stimme">
					<h2 id="h-stimme">My voice</h2>
					<p class="hinweis">What is spoken and how the live text writes it, also when dictating.</p>
					<div class="schalter"><StimmeWahl /></div>
				</section>

			{#if diktat?.supported}
				{@const dk = diktat}
				{@const kuerzel = dk.keys.find((k) => k.key === dk.key)?.label}
				<section id="diktieren" class="blatt" aria-labelledby="h-diktat">
					<h2 id="h-diktat">Dictate into any app</h2>
					<p class="hinweis">
						{#if kuerzel}
							Press <kbd>{kuerzel}</kbd> in any app and speak, then press it again.
							<kbd>Esc</kbd> cancels. A small window above the taskbar shows that Transcribe is listening and what it has
							understood, like a subtitle. Clicking it opens the dictation on the Live page.
						{:else}
							With a keyboard shortcut you dictate into any app: text that appears where the cursor is.
						{/if}
					</p>
					<div class="schalter">
						<label class="feldname">
							Keyboard shortcut
							<select class="feld" value={dk.key} onchange={(e) => diktatSetzen({ key: e.currentTarget.value })}>
								{#each dk.keys as k (k.key)}<option value={k.key}>{k.label}</option>{/each}
								<option value="off">{/* @wc-include */ 'Off'}</option>
							</select>
						</label>
						<Toggle
							checked={dk.insert}
							disabled={dk.key === 'off'}
							title="Insert at the cursor"
							description={dk.insert
								? /* @wc-include */ 'The text appears where the cursor is.'
								: /* @wc-include */ 'Off: the text is not inserted anywhere.'}
							onclick={() => diktatSetzen({ insert: !dk.insert })}
						/>
						<Toggle
							checked={dk.clipboard}
							disabled={dk.key === 'off'}
							title="Copy to clipboard"
							description={dk.clipboard
								? /* @wc-include */ 'The text is then on the clipboard, for Ctrl + V.'
								: /* @wc-include */ 'Off: the clipboard stays as it was.'}
							onclick={() => diktatSetzen({ clipboard: !dk.clipboard })}
						/>
					</div>
					{#if dk.key !== 'off' && !dk.insert && !dk.clipboard}
						<p class="hinweis">
							Both off: the dictation only appears in the small window and stays on the Live page, as a note or for copying by
							hand.
						</p>
					{/if}
					<p class="hinweis">
						Dictation uses the microphone and language set on the <a href="/live">Live</a> page. Every dictation then appears
						there too.
					</p>
					{#if dk.error}<p class="warnung">{dk.error}</p>{/if}
					{#if diktatFehler}<p class="warnung">{diktatFehler}</p>{/if}
				</section>
			{/if}
			{/if}

			{#if offen.id === 'system'}
			<section id="sprache" class="blatt" aria-labelledby="h-sprache">
				<h2 id="h-sprache">Language</h2>
				<label class="feldname" for="oberflaechensprache">Language of the interface</label>
				<Oberflaechensprache id="oberflaechensprache" />
				<p class="hinweis">Until you choose one, Transcribe follows the language of Windows.</p>
			</section>

			<section id="gpu" class="blatt" aria-labelledby="h-gpu">
				<h2 id="h-gpu">Graphics card</h2>
				{#if !gpu}
					<p class="leer">Checking the graphics card…</p>
				{:else if !gpu.gpu.present}
					<p>No NVIDIA GPU detected. Recognition runs on the processor (CPU).</p>
					<p class="hinweis">GPU acceleration is only available with NVIDIA graphics cards (CUDA) on Windows and Linux.</p>
				{:else}
					{@const g = gpu.gpu}
					{@const laedt = imGang(nvidia) ? nvidia : imGang(cuda) ? cuda : null}
					<p class="karte">
						<IconMaterialSymbolsMemoryOutlineRounded aria-hidden="true" />
						<strong>{g.name}</strong>
						<span class="meta">{Math.round(g.memoryMB / 1024)} GB · Driver {g.driver}</span>
					</p>
					{#if !g.supported}
						<p class="hinweis">There is no suitable GPU runtime for this graphics card or system.</p>
					{:else if laedt}
						{@const was = laedt === nvidia ? /* @wc-include */ 'Downloading libraries from NVIDIA' : /* @wc-include */ 'Downloading the GPU runtime'}
						{@const schon = laedt.done ? groesse(laedt.done) : '0 MB'}
						<p>
							{laedt.total
								? /* @wc-include */ `${was} – ${schon} of ${groesse(laedt.total)}`
								: /* @wc-include */ `${was}…`}
						</p>
						<div class="balken" role="progressbar" aria-label={was} aria-valuenow={prozent(laedt)} aria-valuemin="0" aria-valuemax="100">
							<i style:width="{prozent(laedt)}%"></i>
						</div>
						{#if laedt === nvidia && imGang(cuda)}<p class="hinweis">The GPU runtime follows after that.</p>{/if}
					{:else if !g.ready}
						{@const toolkit = g.cudaMajor === 13 ? 'https://developer.nvidia.com/cuda-downloads' : 'https://developer.nvidia.com/cuda-toolkit-archive'}
						<p class="warnung">The GPU is missing libraries from NVIDIA: {g.missing.join(', ')}.</p>
						{#if g.fetch}
							<p class="hinweis">
								The app can download them from NVIDIA itself, about {groesse(g.fetch)}{#if !gpu.installed}, plus the GPU runtime
									of about 500 MB{/if}. This happens once. By choosing “{gpu.installed ? /* @wc-include */ 'Download libraries' : /* @wc-include */ 'Set up GPU'}”, you accept NVIDIA’s license terms for the
								<a href="https://docs.nvidia.com/cuda/eula/" target="_blank" rel="noreferrer">CUDA Toolkit</a> and for
								<a href="https://docs.nvidia.com/deeplearning/cudnn/latest/reference/eula.html" target="_blank" rel="noreferrer">cuDNN</a>.
							</p>
							<div class="handeln">
								<Button
									type="primary"
									size="large"
									loading={fordert.includes('nvidia')}
									onclick={() => (gpu?.installed ? herunterladen('nvidia') : gpuLaden())}
								>
									{#snippet iconLeft()}<IconMaterialSymbolsDownloadRounded />{/snippet}
									{gpu.installed ? /* @wc-include */ 'Download libraries' : /* @wc-include */ 'Set up GPU'}
								</Button>
								<Button type="secondary" size="large" loading={pruefe} onclick={erneutPruefen}>
									{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
									Check again
								</Button>
							</div>
							{#if fehler.nvidia || gpuFehler}
								<p class="warnung">{fehler.nvidia || gpuFehler}</p>
							{:else if nvidia?.state === 'error'}
								<div>
									<p class="warnung">Download failed. Is the computer connected to the internet?</p>
									<details>
										<summary>Technical details</summary>
										<p class="technik">{nvidia.error}</p>
									</details>
								</div>
							{/if}
							<p class="hinweis">
								Rather install them yourself? The
								<a href={toolkit} target="_blank" rel="noreferrer">CUDA Toolkit {g.cudaMajor}</a> and
								<a href="https://developer.nvidia.com/cudnn-downloads" target="_blank" rel="noreferrer">cuDNN 9</a> are available from NVIDIA;
								afterwards, choose “Check again” here.
							</p>
						{:else}
							<p class="hinweis">
								Install the
								<a href={toolkit} target="_blank" rel="noreferrer">CUDA Toolkit {g.cudaMajor}</a> and
								<a href="https://developer.nvidia.com/cudnn-downloads" target="_blank" rel="noreferrer">cuDNN 9</a> from
								NVIDIA, then check again here.
							</p>
							<div class="handeln">
								<Button type="secondary" size="large" loading={pruefe} onclick={erneutPruefen}>
									{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
									Check again
								</Button>
							</div>
						{/if}
					{:else if !gpu.installed}
						<p class="hinweis">
							The GPU runtime (CUDA {g.cudaMajor}) is about 500 MB and is downloaded once. Without it, everything runs on the
							processor.
						</p>
						<div class="handeln">
							<Button type="primary" size="large" onclick={gpuLaden}>
								{#snippet iconLeft()}<IconMaterialSymbolsDownloadRounded />{/snippet}
								Download the GPU runtime
							</Button>
						</div>
						{#if cuda?.state === 'error'}<p class="warnung">{cuda.error}</p>{/if}
						{#if gpuFehler}<p class="warnung">{gpuFehler}</p>{/if}
					{:else}
						<Toggle
							checked={gpu.accel === 'cuda'}
							title="Compute on the GPU (CUDA)"
							description="Off: recognition runs on the processor."
							onclick={() => mitGpu(gpu?.accel !== 'cuda')}
						/>
						<p>
							Currently in use: <strong>{gpu.active === 'cuda' ? 'GPU' : /* @wc-include */ 'Processor (CPU)'}</strong>.
							{#if gpu.restartNeeded}<span class="warnung">The change takes effect when you restart the app.</span>{/if}
						</p>
					{/if}
				{/if}
			</section>

			<section id="speicher" class="blatt" aria-labelledby="h-speicher">
				<h2 id="h-speicher">Storage and models</h2>
				<p class="hinweis">
					Everything is in the data folder: recordings, results, models and settings. To back up, copying this folder is enough.
					Where the EXE is doesn’t matter.
				</p>
				{#if ort}
					<h3 class="teil">Location of the data folder</h3>
					<p class="ortzeile">
						<span class="pfad">{ort.dataDir}</span>
						{#if amStandard}<span class="zustand da">Default</span>{/if}
					</p>
					{#if ort.fixed}
						<p class="hinweis">
							Set at startup with <code>--data</code> or <code>TRANSCRIBE_DATA</code>, so it can’t be changed here.
						</p>
					{:else if umzug?.state === 'restarting' || (ortUnterwegs && (offline || !umzug))}
						<!-- kopiert ist es oft zwischen zwei Abfragen, dann ist die App schon beim Neustart -->
						<p role="status">The data has moved. Transcribe is restarting…</p>
					{:else if umzug?.state === 'copying'}
						<div class="umzug" role="status">
							<p>Moving the data to <span class="pfad">{umzug.target}</span>: {groesse(umzug.done)} of {groesse(umzug.total)}</p>
							<div
								class="balken"
								role="progressbar"
								aria-label="Moving the data folder"
								aria-valuenow={umzug.total ? Math.round((umzug.done * 100) / umzug.total) : 0}
								aria-valuemin="0"
								aria-valuemax="100"
							>
								<i style:width="{umzug.total ? Math.round((umzug.done * 100) / umzug.total) : 0}%"></i>
							</div>
						</div>
					{:else if ortZiel}
						<div class="umzug">
							<h4>Move the data folder?</h4>
							<p>
								The data ({groesse(ortZiel.size)}) moves to <span class="pfad">{ortZiel.target}</span>. Then Transcribe restarts
								and deletes the old folder.
							</p>
							<div class="handeln">
								<Button type="primary" size="large" loading={ortLaeuft === 'umziehen'} onclick={umziehen}>
									{#snippet iconLeft()}<IconMaterialSymbolsDriveFileMoveOutlineRounded />{/snippet}
									Move
								</Button>
								<Button type="secondary" size="large" onclick={() => (ortZiel = null)}>Cancel</Button>
							</div>
						</div>
					{:else}
						{#if !ort.pick}
							<div class="schluesselzeile">
								<input class="feld" spellcheck="false" placeholder="New location, e.g. /mnt/data/transcribe" bind:value={ortEingabe} aria-label="New location of the data folder" />
							</div>
						{/if}
						<div class="handeln">
							<Button
								type="secondary"
								size="large"
								loading={ortLaeuft === 'waehlen' || ortLaeuft === 'pruefen'}
								disabled={!ort.pick && !ortEingabe.trim()}
								onclick={() => (ort.pick ? ortWaehlen() : ortPruefen(ortEingabe.trim()))}
							>
								{#snippet iconLeft()}<IconMaterialSymbolsDriveFileMoveOutlineRounded />{/snippet}
								Choose another location…
							</Button>
							{#if !amStandard}
								<Button type="secondary" size="large" loading={ortLaeuft === 'standard'} onclick={() => ortPruefen('')}>
									Back to the default location
								</Button>
							{/if}
						</div>
						{#if ortFertig}<p role="status">The data is now in <span class="pfad">{ortFertig}</span>.</p>{/if}
					{/if}
					{#if umzug?.state === 'error'}<p class="warnung">{umzug.error}</p>{/if}
					{#if ortFehler}<p class="warnung">{ortFehler}</p>{/if}
				{/if}
				<h3 class="teil">Storage locations</h3>
				{#if offline && !ortUnterwegs}
					<p class="leer fehler">No connection to Transcribe. Is the app still running?</p>
				{:else if !stand}
					<p class="leer">Loading…</p>
				{:else}
					<ul class="liste" aria-label="Storage locations">
						{#each stand.folders as f (f.key)}
							{@const name = ORDNER[f.key] || f.key}
							<li class="eintrag">
								<div class="was">
									<span class="name">{name}</span>
									<span class="pfad">{f.path}</span>
								</div>
								<span class="groesse">{groesse(f.size)}</span>
								<div class="tun">
									<Button type="secondary" size="large" aria-label="Open {name}" onclick={() => oeffnen(f.key)}>
										{#snippet iconLeft()}<IconMaterialSymbolsFolderOpenOutlineRounded />{/snippet}
										Open
									</Button>
								</div>
								{#if fehler[f.key]}<p class="fehler voll">{fehler[f.key]}</p>{/if}
							</li>
						{/each}
					</ul>
				{/if}

				<h3 class="teil">Models and components</h3>
				<p class="hinweis">
					{#if erkennung?.recognition === 'api'}
						Recognition runs through the API, so the app downloads no models, only ffmpeg for the waveform and export. Whatever
						is already there stays.
					{:else}
						The app downloads them on first start. Anything marked “on demand” is only added when it is needed, such as Whisper
						turbo for other languages.
					{/if}
				</p>
				{#if stand}
					<ul class="liste" aria-label="Models and components">
						{#each komponenten as c (c.name)}
							{@const laedt = c.state === 'downloading'}
							<li class="eintrag">
								<div class="was">
									<span class="name">{c.label || c.name}</span>
									{#if c.device}<span class="geraet">runs on the graphics card: {c.device}</span>{/if}
									<span class="pfad">{c.path}</span>
								</div>
								<span class="groesse">{laedt && c.total ? `${groesse(c.done)} / ${groesse(c.total)}` : groesse(c.size)}</span>
								<div class="tun">
									{#if laedt}
										<span class="zustand laedt">downloading {c.total ? `${prozent(c)} %` : '…'}</span>
									{:else if c.state === 'error'}
										<span class="zustand fehlt">Error</span>
									{:else if c.installed}
										<span class="zustand da"><IconMaterialSymbolsCheckRounded aria-hidden="true" />installed</span>
									{:else}
										<span class="zustand" class:fehlt={!c.onDemand}>{c.onDemand ? /* @wc-include */ 'on demand' : /* @wc-include */ 'missing'}</span>
									{/if}
									{#if !c.installed && !laedt}
										<Button
											type="secondary"
											size="large"
											loading={fordert.includes(c.name)}
											aria-label={c.state === 'error'
												? /* @wc-include */ `Download ${c.label || c.name} again`
												: /* @wc-include */ `Download ${c.label || c.name}`}
											onclick={() => herunterladen(c.name)}
										>
											{#snippet iconLeft()}<IconMaterialSymbolsDownloadRounded />{/snippet}
											{c.state === 'error' ? /* @wc-include */ 'Download again' : /* @wc-include */ 'Download'}
										</Button>
									{/if}
								</div>
								{#if laedt && c.total}
									<div class="balken voll" role="progressbar" aria-label="Downloading {c.label || c.name}" aria-valuenow={prozent(c)} aria-valuemin="0" aria-valuemax="100">
										<i style:width="{prozent(c)}%"></i>
									</div>
								{/if}
								{#if fehler[c.name]}
									<p class="fehler voll">{fehler[c.name]}</p>
								{:else if c.error}
									<!-- die Meldung des Servers ist technisch (URL, Proxy); zuerst ein Satz, der weiterhilft -->
									<div class="voll">
										<p class="fehler">Download failed. Is the computer connected to the internet?</p>
										<details>
											<summary>Technical details</summary>
											<p class="technik">{c.error}</p>
										</details>
									</div>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section id="programm" class="blatt" aria-labelledby="h-programm">
				<h2 id="h-programm">About Transcribe</h2>
				<div class="herkunft">
					<MoliriLogo class="herkunft-zeichen" />
					<p>
						Transcribe by Moliri is free and runs on its own. It is also the companion app to the museum database
						<a href={MOLIRI} target="_blank" rel="noopener">Moliri</a>. How everything works is explained in the
						<a href={HANDBUCH} target="_blank" rel="noopener">manual</a>.
					</p>
				</div>
				{#if update.stand}
					{@const u = update.stand}
					{@const neu = versionText(u.latest)}
					<p class="karte">
						<strong>Version {versionText(u.version)}</strong>
						{#if u.checks && u.state === 'current'}
							<span class="meta"
								>{u.checked ? /* @wc-include */ `the latest, last checked ${wann(u.checked)}` : /* @wc-include */ 'the latest'}</span
							>
						{/if}
					</p>
					{#if vorher}
						<p class="gespeichert" role="status">Updated from version {versionText(vorher)} to {versionText(u.version)}.</p>
					{/if}
					{#if u.manual}
						<p class="hinweis">
							On Linux and macOS, Transcribe does not update itself. New versions are available under
							<a href="https://github.com/moliri-app/transcribe/releases" target="_blank" rel="noreferrer">Releases</a>.
						</p>
					{:else if !u.checks}
						<p class="hinweis">
							A development version does not check for updates. Finished versions are available under
							<a href="https://github.com/moliri-app/transcribe/releases" target="_blank" rel="noreferrer">Releases</a>.
						</p>
					{:else if u.state === 'available' || u.state === 'failed'}
						<div class="update" role="group" aria-labelledby="h-update">
							<h3 id="h-update">Version {neu} is available</h3>
							<p class="hinweis">
								Transcribe downloads it, replaces the program and restarts. Running jobs and recordings finish first; your
								recordings and settings stay as they are.
								{#if u.page}<a href={u.page} target="_blank" rel="noreferrer">What’s new?</a>{/if}
							</p>
							<div class="handeln">
								<Button type="primary" size="large" loading={updateLaeuft === 'install'} onclick={() => updateTun('install')}>
									{#snippet iconLeft()}<IconMaterialSymbolsUpgradeRounded />{/snippet}
									{u.state === 'failed' ? /* @wc-include */ 'Try again' : /* @wc-include */ 'Update now'}
								</Button>
							</div>
							{#if u.state === 'failed'}<p class="warnung">{u.error}</p>{/if}
						</div>
					{:else if u.state === 'downloading'}
						<p>Downloading version {neu}…</p>
						<div
							class="balken"
							role="progressbar"
							aria-label="Downloading version {neu}"
							aria-valuenow={Math.round((u.progress ?? 0) * 100)}
							aria-valuemin="0"
							aria-valuemax="100"
						>
							<i style:width="{Math.round((u.progress ?? 0) * 100)}%"></i>
						</div>
					{:else if u.state === 'waiting'}
						<p role="status">
							Version {neu} is installed. Transcribe restarts as soon as running jobs and recordings are finished.
						</p>
					{:else if u.state === 'restarting' || update.weg}
						<p role="status">Transcribe is restarting…</p>
					{:else}
						<div class="handeln">
							<Button type="secondary" size="large" loading={updateLaeuft === 'check'} onclick={() => updateTun('check')}>
								{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
								Check for updates
							</Button>
						</div>
					{/if}
					{#if updateFehler}<p class="warnung">{updateFehler}</p>{/if}
				{/if}
				{#if stand}
					<dl class="angaben">
						<dt>Program</dt>
						<dd class="pfad">{stand.exe}</dd>
						<dt>Log</dt>
						<dd class="pfad">{stand.log}</dd>
						<dt>System</dt>
						<dd>{stand.os}</dd>
						<dt>sherpa-onnx</dt>
						<dd>{stand.sherpa}</dd>
					</dl>
				{/if}
				<a class="weiter" href="/lizenzen">
					Licenses and third-party components <IconMaterialSymbolsArrowForwardRounded aria-hidden="true" />
				</a>
			</section>
			{/if}
		</div>
	</div>
</main>

<style>
	main {
		max-width: 76rem;
		margin: 0 auto;
		padding: 2rem 1.5rem 4rem;
		display: grid;
		gap: 1.5rem;
	}

	/* Menü links, die Abschnitte rechts; das Menü bleibt beim Scrollen stehen */
	.rahmen {
		display: grid;
		grid-template-columns: 14rem minmax(0, 1fr);
		gap: 2rem;
		align-items: start;
	}
	.menue {
		position: sticky;
		top: calc(60px + 1.5rem);
	}
	.menue ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.125rem;
	}
	.menue a {
		display: flex;
		align-items: center;
		min-height: var(--hit);
		padding: 0 1rem;
		border-radius: 2rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
		text-decoration: none;
		white-space: nowrap;
		transition: background-color 0.15s, color 0.15s;
	}
	.menue a:hover {
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
	}
	/* die offene Kategorie: Tintenpille wie der aktive Tab der App-Leiste, umgekehrt */
	.menue a[aria-current='page'] {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	/* ihre Abschnitte darunter, eingerückt an einer Linie; der, wo man ist, kräftig mit grüner Marke */
	.menue .unter {
		margin: 0.25rem 0 0.5rem 1.25rem;
		padding-left: 0.5rem;
		border-left: 2px solid hsl(var(--color-border-primary));
		gap: 0;
	}
	.menue .unter a {
		min-height: 2.25rem;
		padding: 0 0.75rem;
		font-weight: 500;
	}
	.menue .unter a[aria-current='location'] {
		background: none;
		color: hsl(var(--color-text-primary));
		font-weight: 700;
		box-shadow: inset 3px 0 0 hsl(var(--color-green-100));
		border-radius: 0 2rem 2rem 0;
	}
	.abschnitte {
		display: grid;
		gap: 1.5rem;
		max-width: 56rem;
	}
	.abschnitte > section {
		scroll-margin-top: calc(60px + 1.5rem);
	}
	h3.teil {
		margin-top: 0.75rem;
		font-size: 1.0625rem;
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

	.blatt {
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		padding: 1.5rem 1.75rem 1.75rem;
		box-shadow: 0 1px 2px hsl(var(--color-shadow)), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
		display: grid;
		gap: 1rem;
		align-content: start;
	}
	h2 {
		font-size: 1.25rem;
	}
	/* Überschrift und das, was sie erklärt, rücken zusammen */
	h2 + .hinweis,
	.zeile + .hinweis {
		margin-top: -0.5rem;
	}
	.hinweis {
		color: hsl(var(--color-text-secondary));
		max-width: 68ch;
	}
	.hinweis a {
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		text-decoration-thickness: 1px;
	}
	.hinweis a:hover {
		text-decoration-thickness: 2px;
	}
	.meta {
		color: hsl(var(--color-text-secondary));
		font-size: 0.9375rem;
	}
	.leer {
		color: hsl(var(--color-text-secondary));
	}
	.fehler,
	.warnung {
		color: hsl(var(--color-text-error));
		font-weight: 600;
	}

	.zeile {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.25rem 1rem;
	}
	.gespeichert {
		min-height: 1.5em;
		font-size: 0.9375rem;
		font-weight: 600;
		color: hsl(var(--color-text-success));
	}
	.gespeichert.fehler {
		color: hsl(var(--color-text-error));
	}
	.vorgaben {
		min-inline-size: 0;
		margin: 0;
		padding: 0;
		border: 0;
	}

	/* was Sprecher und Sprachen bewirken: eine ruhige Begriffsliste unter den Feldern */
	.erklaerung {
		display: grid;
		grid-template-columns: max-content minmax(0, 1fr);
		gap: 0.4rem 1.25rem;
		max-width: 44rem;
		margin: 0;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.erklaerung dt {
		font-weight: 700;
		color: hsl(var(--color-text-primary));
	}
	.erklaerung dd {
		margin: 0;
	}

	.ablegen {
		display: grid;
		gap: 0.3rem;
		max-width: 36rem;
		padding-top: 1rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}

	/* Erkennung: die drei Wege als große Optionsfelder */
	.arten {
		display: grid;
		gap: 0.5rem;
		max-width: 44rem;
		margin: 0;
		padding: 0;
		border: 0;
	}
	.art {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 0.85rem 1rem;
		border: 1px solid hsl(var(--color-border-primary));
		border-radius: 0.6rem;
		cursor: pointer;
		transition: border-color 0.15s;
	}
	.art:hover {
		border-color: hsl(var(--color-border-secondary));
	}
	.art.gewaehlt {
		border-color: hsl(var(--color-neutral-100));
		box-shadow: inset 0 0 0 1px hsl(var(--color-neutral-100));
	}
	.art input {
		margin-top: 0.15rem;
	}
	.art > span {
		display: grid;
		gap: 0.15rem;
	}
	.api {
		display: grid;
		gap: 0.4rem;
		max-width: 44rem;
	}
	.apifelder {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		gap: 1rem 1.25rem;
	}
	.adresse {
		flex: 1 1 16rem;
	}
	.api .hinweis {
		margin-bottom: 0.6rem;
	}
	.schluesselzeile {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
	}
	.schluesselzeile .feld {
		flex: 1 1 18rem;
	}

	.schalter {
		display: grid;
		gap: 0.75rem;
		max-width: 36rem;
	}
	.schalter .feld {
		justify-self: start;
		min-width: 18rem;
	}
	kbd {
		padding: 0.05rem 0.4rem;
		border: 1px solid hsl(var(--color-border-secondary));
		border-bottom-width: 2px;
		border-radius: 0.35rem;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
		font: inherit;
		font-size: 0.9375rem;
		font-weight: 600;
		white-space: nowrap;
	}

	.karte {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 0.6rem;
	}
	.karte :global(svg) {
		width: 1.4rem;
		height: 1.4rem;
		color: hsl(var(--color-text-secondary));
	}

	.handeln {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
	}

	/* Ordner und Komponenten: Name mit Pfad darunter, Größe, Zustand und Knopf rechts */
	.liste {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.eintrag {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto auto;
		align-items: center;
		gap: 0.4rem 1.25rem;
		padding: 0.75rem 0;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	.eintrag:first-child {
		border-top: 0;
		padding-top: 0.25rem;
	}
	.was {
		display: grid;
		gap: 0.1rem;
		min-width: 0;
	}
	.name {
		font-weight: 700;
	}
	.pfad {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		overflow-wrap: anywhere;
	}
	.geraet {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-primary));
	}
	.groesse {
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-secondary));
		text-align: right;
		white-space: nowrap;
	}
	.tun {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.75rem;
		min-height: var(--hit);
	}
	.voll {
		grid-column: 1 / -1;
	}
	details summary {
		min-height: 2rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.technik {
		margin-top: 0.25rem;
		padding: 0.6rem 0.9rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		overflow-wrap: anywhere;
	}

	/* Zustand als Pille wie in der Bibliothek */
	.zustand {
		display: inline-flex;
		align-items: center;
		gap: 0.2rem;
		padding: 0.1rem 0.65rem;
		border-radius: 999px;
		font-size: 0.875rem;
		font-weight: 700;
		white-space: nowrap;
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-secondary));
	}
	.zustand :global(svg) {
		width: 1rem;
		height: 1rem;
	}
	.zustand.da {
		background: hsl(var(--color-green-10));
		color: hsl(var(--color-text-success));
	}
	.zustand.laedt {
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
		font-variant-numeric: tabular-nums;
	}
	.zustand.fehlt {
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
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

	.ortzeile {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem 0.75rem;
	}
	.ortzeile .pfad {
		color: hsl(var(--color-text-primary));
		font-weight: 600;
	}
	/* der Umzug des Datenordners: abgesetzt wie eine Karte im Blatt */
	.umzug {
		display: grid;
		gap: 0.75rem;
		max-width: 44rem;
		padding: 1rem 1.25rem 1.25rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-secondary));
	}
	.umzug h4 {
		font-size: 1.0625rem;
	}

	/* eine neuere Version: abgesetzt wie eine Karte im Blatt */
	.update {
		display: grid;
		gap: 0.75rem;
		padding: 1rem 1.25rem 1.25rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-green-10));
	}
	.update h3 {
		font-size: 1.0625rem;
	}

	.angaben {
		display: grid;
		grid-template-columns: max-content minmax(0, 1fr);
		gap: 0.5rem 1.5rem;
		margin: 0;
	}
	.angaben dt {
		color: hsl(var(--color-text-secondary));
	}
	.angaben dd {
		margin: 0;
	}
	.angaben dd.pfad {
		font-size: 1rem;
		color: hsl(var(--color-text-primary));
	}

	.weiter {
		justify-self: start;
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		min-height: var(--hit);
		font-weight: 700;
		text-decoration: underline;
		text-underline-offset: 0.2em;
		text-decoration-thickness: 2px;
		text-decoration-color: hsl(var(--color-green-100));
	}
	.weiter:hover {
		text-decoration-color: currentColor;
	}

	/* Über Transcribe: woher es kommt, mit Moliris Zeichen */
	.herkunft {
		display: flex;
		align-items: flex-start;
		gap: 1rem;
	}
	.herkunft :global(.herkunft-zeichen) {
		width: 2rem;
		height: 2rem;
		margin-top: 0.125rem;
	}
	.herkunft p {
		margin: 0;
		max-width: 62ch;
	}
	.herkunft a {
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}

	/* Programme: wer fragt, auf einen Blick abgesetzt */
	.fragen {
		display: grid;
		gap: 0.5rem;
		padding: 0.75rem 1.25rem 0.5rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-green-10));
	}
	.fragen h3.teil {
		margin-top: 0.25rem;
	}
	.apiadresse {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1rem;
	}
	.apiadresse code,
	.modi code,
	.protokoll code,
	.protokoll pre {
		font-family: var(--font-mono);
		font-size: 0.9375em;
	}
	.apiadresse code {
		padding: 0.55rem 0.9rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 1.0625rem;
		overflow-wrap: anywhere;
	}
	.modi {
		display: grid;
		grid-template-columns: 12rem minmax(0, 1fr);
		gap: 0.75rem 1.5rem;
		margin: 0;
		padding: 1rem 0;
		border-block: 1px solid hsl(var(--color-border-primary));
	}
	.modi dt {
		font-weight: 700;
	}
	.modi dt code {
		display: block;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
	.modi dd {
		margin: 0;
		max-width: 62ch;
	}
	.modi a,
	.meta a {
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
	}
	.protokoll {
		display: grid;
		gap: 0.75rem;
		max-width: 70ch;
	}
	.protokoll pre {
		margin: 0;
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-size: 0.875rem;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	/* Texterkennung: was installiert ist, dann ein Bild zum Ausprobieren */
	.leser {
		display: grid;
		gap: 0.25rem;
	}
	.leser code {
		font-family: var(--font-mono);
		font-size: 0.9375rem;
	}
	.probe {
		display: grid;
		gap: 1rem;
		padding-top: 1rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	.bildwahl {
		justify-self: start;
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
	.bildwahl:hover {
		background: color-mix(in oklab, hsl(var(--color-neutral-100)) 80%, hsl(var(--color-neutral-0)));
	}
	.bildwahl:focus-within {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.bildwahl.liest {
		cursor: progress;
		opacity: 0.75;
	}
	.bildwahl :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}
	.ergebnis {
		display: grid;
		/* das Bild groß genug, dass man die Kästen der Wörter erkennt */
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 1.25rem;
		align-items: start;
	}
	.seite {
		width: 100%;
		max-height: 28rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
	}
	.seite rect {
		fill: hsl(var(--color-blue-100) / 0.12);
		stroke: hsl(var(--color-blue-200));
		stroke-width: 1.5px;
		vector-effect: non-scaling-stroke;
	}
	.gelesen {
		display: grid;
		gap: 0.4rem;
		min-width: 0;
	}
	.gelesen pre {
		max-height: 20rem;
		overflow: auto;
		margin: 0;
		padding: 0.875rem 1rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-family: var(--font-ui);
		font-size: 1rem;
		line-height: 1.6;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	/* schmal: das Menü wird eine Leiste über den Abschnitten, die mitläuft und seitlich scrollt */
	@media (max-width: 900px) {
		.rahmen {
			grid-template-columns: minmax(0, 1fr);
			gap: 1rem;
		}
		.menue {
			top: 60px;
			z-index: 2;
			margin: 0 -1.5rem;
			padding: 0.5rem 1.5rem;
			background: hsl(var(--page-bg));
			overflow-x: auto;
			scrollbar-width: none;
		}
		.menue ul {
			display: flex;
			gap: 0.25rem;
		}
		.menue .unter {
			display: none;
		}
		.abschnitte > section {
			scroll-margin-top: calc(60px + 4.5rem);
		}
	}

	@media (max-width: 640px) {
		main {
			padding: 1.25rem 0.75rem 3rem;
		}
		.menue {
			margin: 0 -0.75rem;
			padding: 0.5rem 0.75rem;
		}
		.ergebnis {
			grid-template-columns: minmax(0, 1fr);
		}
		.modi {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.25rem;
		}
		.modi dd {
			margin-bottom: 0.75rem;
		}
		.erklaerung {
			grid-template-columns: minmax(0, 1fr);
		}
		.blatt {
			padding: 1.25rem 1rem;
		}
		.eintrag {
			grid-template-columns: minmax(0, 1fr) auto;
		}
		.tun {
			grid-column: 1 / -1;
			justify-content: flex-start;
		}
		.angaben {
			grid-template-columns: 1fr;
			gap: 0 0;
		}
		.angaben dd {
			margin-bottom: 0.6rem;
		}
	}
</style>
