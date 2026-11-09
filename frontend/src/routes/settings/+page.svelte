<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Einstellungen, links ein Menü zu den Abschnitten (schmal eine Leiste oben): wo die Sprache
     erkannt wird (auf diesem Rechner, über eine API oder beides; welche API), Vorgaben für neue
     Aufnahmen (Sprecher und Sprachen, sofort gespeichert), andere Programme („Im Hintergrund bereit“,
     Start mit Windows, Weiterlaufen im Hintergrund), Texterkennung (OCR: die Vorgabe für Programme, der
     KI-Dienst mit eigenem Schlüssel, ein Bild zum Ausprobieren), Diktieren
     per Tastenkürzel, GPU-Beschleunigung, Speicherorte, Modelle und Komponenten samt Download, Angaben
     zum Programm samt Version und Update (internal/update). -->
<script>
	import { onMount, tick } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import AuftragsOptionen from '$lib/AuftragsOptionen.svelte';
	import { einrichtung } from '$lib/Einrichtung.svelte';
	import { ENGINES, einzeln, getJSON, post, sprecherAus, verbindung } from '$lib/api.js';
	import { ladeUpdate, update, updateAktion, versionText, vorherigeVersion } from '$lib/update.svelte.js';

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
		data: 'Datenordner',
		audio: 'Aufnahmen',
		results: 'Ergebnisse',
		models: 'Modelle',
		lib: 'Erkennungs-Runtime',
		bin: 'ffmpeg'
	});


	const groesse = (/** @type {number} */ n) =>
		n >= 1073741824
			? (n / 1073741824).toFixed(2).replace('.', ',') + ' GB'
			: n >= 1048576
				? Math.round(n / 1048576) + ' MB'
				: n > 0
					? Math.max(1, Math.round(n / 1024)) + ' KB'
					: '–';
	const prozent = (/** @type {Komponente} */ c) => (c.total ? Math.round((c.done * 100) / c.total) : 0);

	// ---- Erkennung ----
	const ARTEN = [
		['local', 'Nur auf diesem Rechner', 'Die Modelle laufen hier. Keine Aufnahme verlässt den Rechner.'],
		[
			'api',
			'Nur über eine API',
			'Die Aufnahmen gehen zur Erkennung an einen Dienst, im Internet oder im eigenen Netz. Die App lädt dafür keine Modelle.'
		],
		['both', 'Beides', 'Du legst fest, was Live und ganze Aufnahmen nutzen, und wählst bei jedem Auftrag neu, wenn du willst.']
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
			erkennungFehler = 'Nicht gespeichert: ' + /** @type {Error} */ (e).message;
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
			schluesselStand = wert ? 'Schlüssel gespeichert.' : 'Schlüssel entfernt.';
		}
		speichertSchluessel = false;
	}

	// ---- Verbundene Ordner (internal/server/folders.go) ----
	/** @type {{ folders: { path: string, name: string, exists: boolean }[], sidecar: boolean } | null} */
	let verbunden = $state(null);
	let nebenAn = $state(true);
	let ordnerStand = $state('');

	async function ladeVerbunden() {
		verbunden = await getJSON('/api/folders');
		nebenAn = verbunden?.sidecar ?? true;
	}

	/** @param {{ action: 'remove' | 'sidecar', path?: string, on?: boolean }} body */
	async function verbundenAendern(body) {
		ordnerStand = '';
		try {
			verbunden = await post('/api/folders', body);
			nebenAn = verbunden?.sidecar ?? true;
			ordnerStand = body.action === 'remove' ? 'Getrennt. Die Aufnahmen und Transkripte im Ordner bleiben, wo sie sind.' : 'Gespeichert.';
		} catch (e) {
			ordnerStand = 'Nicht gespeichert: ' + /** @type {Error} */ (e).message;
		}
	}

	// ---- Vorgaben ----
	/** @type {import('$lib/api.js').Optionen} */
	let optionen = $state({ speakers: '', language: 'auto' });
	let sofort = $state(false);
	let geladen = $state(false);
	let gespeichert = $state('');
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
		try {
			await post('/api/prefs', patch);
			gespeichert = 'Gespeichert.';
			gespeichertUhr = setTimeout(() => (gespeichert = ''), 2000);
		} catch (e) {
			gespeichert = 'Nicht gespeichert: ' + /** @type {Error} */ (e).message;
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

	/** die Modelle von Tesseract mit Namen (models/status.go) */
	const OCR_SPRACHEN = /** @type {Record<string, string>} */ ({
		deu: 'Deutsch',
		eng: 'Englisch',
		frk: 'Fraktur (frk)',
		Fraktur: 'Fraktur-Schrift',
		frak2021: 'Fraktur (frak2021)',
		latin: 'Lateinische Schrift (Deutsch, Englisch, …)',
		fra: 'Französisch',
		ita: 'Italienisch',
		spa: 'Spanisch',
		nld: 'Niederländisch',
		pol: 'Polnisch',
		ces: 'Tschechisch',
		dan: 'Dänisch',
		swe: 'Schwedisch',
		nor: 'Norwegisch',
		rus: 'Russisch'
	});
	/** was jede Erkennung gut kann (ocr) */
	const OCR_LESER = /** @type {Record<string, string>} */ ({
		tesseract: 'Liest Druck und Schreibmaschine, mit eigenen Modellen für Fraktur.',
		onnx: 'Liest gedruckte lateinische Schrift sehr genau, auch Karteikarten und Listen, und kennt die Lage jedes Worts; auf der Grafikkarte schnell. Für Fraktur ist Tesseract besser.',
		os: 'Die Texterkennung von Windows: sofort da und sehr schnell, mit der Lage jedes Worts, aber ohne Fraktur und Handschrift. Ihre Sprachen kommen aus Windows; weitere fügst du unter Einstellungen › Zeit und Sprache › Sprache hinzu.',
		api: 'Ein KI-Modell liest das Bild, in jeder Sprache und Schrift, auch Handschrift und Kurrent. Es findet die Zeilen, aber nicht die Lage jedes Worts. Das Bild geht dafür an den Dienst, abgerechnet wird über deinen Schlüssel.'
	});
	/** die Sprache, mit der eine Erkennung beim Ausprobieren anfängt
	 * @param {Leser} l */
	const ersteSprache = (l) => {
		const s = ocrReihe([...l.languages, ...l.download]);
		return s.includes('deu') ? 'deu' : (s[0] ?? 'deu');
	};
	const OCR_LAYOUTS = /** @type {Record<string, string>} */ ({
		auto: 'Seite mit Spalten (automatisch)',
		lines: 'Zeilen, etwa Karteikarte oder Formular',
		block: 'Ein Textblock, etwa ein Etikett',
		sparse: 'Verstreut, etwa Schrift auf Objekten'
	});
	const ocrName = (/** @type {string} */ l) => OCR_SPRACHEN[l] ?? l;
	/** in der Reihenfolge oben, unbekannte danach */
	const ocrReihe = (/** @type {string[]} */ l) => {
		const rang = (/** @type {string} */ x) => {
			const i = Object.keys(OCR_SPRACHEN).indexOf(x);
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
			ocrDienstFehler = 'Nicht gespeichert: ' + /** @type {Error} */ (e).message;
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
			ocrSchluesselStand = wert ? 'Schlüssel gespeichert.' : 'Schlüssel entfernt.';
		}
		speichertOcrSchluessel = false;
	}

	// ---- Menü: welcher Abschnitt gerade zu sehen ist ----
	const ABSCHNITTE = /** @type {const} */ ([
		['erkennung', 'Erkennung'],
		['neu', 'Neue Aufnahmen'],
		['ordner', 'Ordner'],
		['programme', 'Andere Programme'],
		['ocr', 'Texterkennung'],
		['diktieren', 'Diktieren'],
		['gpu', 'Grafikkarte'],
		['speicher', 'Speicher und Modelle'],
		['programm', 'Über Transcribe']
	]);
	const menue = $derived(
		ABSCHNITTE.filter(([id]) => (id === 'ocr' ? ocr !== false : id === 'diktieren' ? !!diktat?.supported : true))
	);
	let aktuell = $state('erkennung');
	/** im Menü angeklickt: bleibt markiert, bis man selbst scrollt, auch wenn der Sprung ganz unten endet */
	let angeklickt = '';
	/** @type {HTMLElement | undefined} */
	let menueLeiste = $state();

	// schmal scrollt die Leiste seitlich: der aktuelle Abschnitt bleibt darin zu sehen
	$effect(() => {
		const a = menueLeiste?.querySelector(`a[href="#${aktuell}"]`);
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
				const ids = menue.map(([id]) => id);
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
		const uhr = d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
		return heute ? `heute um ${uhr}` : `am ${d.toLocaleDateString('de-DE')} um ${uhr}`;
	}

	onMount(() => {
		vorher = vorherigeVersion();
		const geladen = Promise.allSettled([
			ladeUpdate(),
			ladeOcr(),
			ladeHintergrund(),
			ladeDiktat(),
			ladeVorgaben().catch(() => (gespeichert = 'Die Vorgaben ließen sich nicht laden.')),
			ladeVerbunden().catch(() => {}),
			ladeStand(),
			ladeGpu()
		]);
		// mit #abschnitt geöffnet (das Update in der App-Leiste, das Menü im Infobereich): der Sprung
		// des Browsers kam, bevor die Abschnitte darüber geladen waren, und landet zu weit oben
		const ziel = decodeURIComponent(location.hash.slice(1));
		if (ziel)
			geladen.then(tick).then(() => {
				const el = document.getElementById(ziel);
				if (!el) return;
				aktuell = angeklickt = ziel;
				el.scrollIntoView();
			});
		const s = setInterval(einzeln(ladeStand), 2000);
		const g = setInterval(einzeln(() => ladeGpu()), 4000);
		return () => {
			clearInterval(s);
			clearInterval(g);
			clearTimeout(gespeichertUhr);
		};
	});
</script>

<svelte:head><title>Einstellungen · Transcribe by Moliri</title></svelte:head>

<main>
	<header class="kopf">
		<h1>Einstellungen</h1>
		<p>Änderungen gelten sofort, speichern musst du nichts.</p>
	</header>

	<div class="rahmen">
		<nav class="menue" aria-label="Abschnitte der Einstellungen" bind:this={menueLeiste}>
			<ul>
				{#each menue as [id, name] (id)}
					<li>
						<a href="#{id}" aria-current={aktuell === id ? 'location' : undefined} onclick={() => (aktuell = angeklickt = id)}>{name}</a>
					</li>
				{/each}
			</ul>
		</nav>

		<div class="abschnitte" {@attach beobachten}>
			<section id="erkennung" class="blatt" aria-labelledby="h-erkennung">
				<h2 id="h-erkennung">Erkennung</h2>
				<p class="hinweis">
					Die Sprache in deinen Aufnahmen erkennen die Modelle auf diesem Rechner oder ein Dienst über seine API, im
					Internet oder im eigenen Netz.
				</p>
				{#if !erkennung}
					<p class="leer">Lade …</p>
				{:else}
					{@const e = erkennung}
					<fieldset class="arten">
						<legend class="sr">Wo die Sprache erkannt wird</legend>
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
									Dienst
									<select class="feld" value={a.provider} onchange={(ev) => apiSetzen({ provider: ev.currentTarget.value })}>
										{#each e.apiProviders as d (d.id)}<option value={d.id}>{d.label}</option>{/each}
									</select>
								</label>
								<label class="feldname adresse">
									Adresse
									<input
										class="feld"
										spellcheck="false"
										value={a.url}
										onchange={(ev) => apiSetzen({ url: ev.currentTarget.value })}
									/>
								</label>
								<label class="feldname">
									Modell
									<input class="feld" spellcheck="false" value={a.model} onchange={(ev) => apiSetzen({ model: ev.currentTarget.value })} />
								</label>
							</div>
							<p class="hinweis">
								{#if dienst.kind === 'openai'}
									Spricht die OpenAI-kompatible Schnittstelle, wie OpenAI, Groq, Mistral oder ein eigener Server (etwa
									Speaches oder LocalAI). Sie unterscheidet keine Sprecher, und Live zeigt den Text erst nach jeder Sprechpause.
								{:else}
									Unterscheidet Sprecher und erkennt Geräusche wie Lachen oder Applaus; Live zeigt den Text schon beim Sprechen.
								{/if}
							</p>

							<label class="feldname" for="schluessel">API-Schlüssel{dienst.noKey ? ' (falls der Server einen verlangt)' : ''}</label>
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
									placeholder={a.keySet ? `hinterlegt${a.keyHint ? ` (endet auf ${a.keyHint})` : ''}` : 'Schlüssel'}
									bind:value={schluessel}
								/>
								<Button type="primary" size="large" nativeType="submit" disabled={!schluessel.trim()} loading={speichertSchluessel}>
									{a.keySet ? 'Ersetzen' : 'Speichern'}
								</Button>
								{#if a.keySet}
									<Button type="secondary" size="large" onclick={() => schluesselSpeichern('')}>Entfernen</Button>
								{/if}
							</form>
							{#if schluesselStand}
								<p class="gespeichert" role="status">{schluesselStand}</p>
							{:else if !a.keySet && !dienst.noKey}
								<p class="warnung">Ohne Schlüssel kann die App {dienst.label} nicht nutzen.</p>
							{/if}
							<p class="hinweis">
								{#if dienst.keys}
									Den Schlüssel legst du in deinem Konto bei
									<a href={dienst.keys} target="_blank" rel="noreferrer">{dienst.label}</a> an. Abgerechnet wird meist nach
									Minuten Audio.
								{/if}
								Der Schlüssel bleibt in den Einstellungen auf diesem Rechner.
							</p>
						</div>
					{/if}

					{#if e.recognition === 'both'}
						<div class="schalter">
							<label class="feldname">
								Live und Diktieren
								<select class="feld" value={e.liveEngine} onchange={(ev) => erkennungSetzen({ liveEngine: ev.currentTarget.value })}>
									{#each Object.entries(ENGINES) as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
								</select>
							</label>
							<label class="feldname">
								Ganze Aufnahmen
								<select class="feld" value={e.fileEngine} onchange={(ev) => erkennungSetzen({ fileEngine: ev.currentTarget.value })}>
									{#each Object.entries(ENGINES) as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
								</select>
							</label>
						</div>
						<p class="hinweis">
							Für ganze Aufnahmen ist das die Vorgabe: In der Bibliothek und beim Ablegen von Dateien wählst du bei jeder
							Aufnahme, was erkennen soll.
						</p>
					{/if}
					{#if erkennungFehler}<p class="warnung">{erkennungFehler}</p>{/if}
				{/if}
			</section>

			<section id="neu" class="blatt" aria-labelledby="h-neu">
				<div class="zeile">
					<h2 id="h-neu">Neue Aufnahmen</h2>
					<p class="gespeichert" class:fehler={gespeichert && gespeichert !== 'Gespeichert.'} role="status">{gespeichert}</p>
				</div>
				<p class="hinweis">
					Damit fängt jede Aufnahme an, die du ins Fenster ziehst oder in der Bibliothek transkribierst. Dort lässt es sich
					für einzelne Aufnahmen ändern.
				</p>

				<!-- gesperrt, bis die Vorgaben da sind: eine Änderung davor ginge verloren -->
				<fieldset class="vorgaben" disabled={!geladen}>
					<AuftragsOptionen bind:optionen onchange={optionenSpeichern} erkennung={erkennung?.recognition} />
				</fieldset>

				<dl class="erklaerung">
					<dt>Sprecher</dt>
					<dd>
						Meist erkennt die App selbst, wie viele Menschen sprechen. Eine feste Zahl hilft, wenn sich Stimmen ähneln oder
						jemand nur kurz spricht.
					</dd>
					<dt>Sprachen</dt>
					<dd>
						Automatisch erkennt die App jede Sprache. Nennst du die Sprachen, die vorkommen, wählt sie nur unter diesen;
						das hilft bei Mundart und kurzen fremdsprachigen Stellen.
					</dd>
				</dl>

				<div class="ablegen">
					<h3 class="feldname">Wenn du Dateien oder Ordner ins Fenster ziehst</h3>
					<Toggle
						bind:checked={sofort}
						disabled={!geladen}
						title="Sofort mit diesen Vorgaben starten"
						description={sofort
							? 'Die Aufnahmen werden gleich transkribiert.'
							: 'Aus: Die App fragt erst nach Sprechern und Sprachen und startet, wenn du bestätigst.'}
						onclick={() => speichern({ dropMode: sofort ? 'auto' : 'ask' })}
					/>
				</div>
			</section>

			<section id="ordner" class="blatt" aria-labelledby="h-ordner">
				<div class="zeile">
					<h2 id="h-ordner">Verbundene Ordner</h2>
					<!-- nur da, wenn es etwas sagt: die Seite hat sonst einen Status, den der übrigen Vorgaben -->
					{#if ordnerStand}<p class="gespeichert" class:fehler={ordnerStand.startsWith('Nicht')} role="status">{ordnerStand}</p>{/if}
				</div>
				<p class="hinweis">
					Die Aufnahmen eines verbundenen Ordners stehen in der Bibliothek, ohne kopiert zu werden, auch die in seinen
					Unterordnern. Verbinden lässt sich ein Ordner in der <a href="/">Bibliothek</a>. Transcribe löscht darin nie etwas.
				</p>
				{#if verbunden?.folders.length}
					<ul class="liste" aria-label="Verbundene Ordner">
						{#each verbunden.folders as f (f.path)}
							<li class="eintrag">
								<div class="was">
									<span class="name">{f.name}</span>
									<span class="pfad">{f.path}{f.exists ? '' : ' (nicht erreichbar)'}</span>
								</div>
								<span></span>
								<div class="tun">
									<Button type="secondary" size="large" aria-label="{f.name} trennen" onclick={() => verbundenAendern({ action: 'remove', path: f.path })}>
										{#snippet iconLeft()}<IconMaterialSymbolsLinkOffRounded />{/snippet}
										Trennen
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
						title="Transkripte neben die Aufnahmen legen"
						description={nebenAn
							? 'Jedes Transkript liegt auch als Datei neben seiner Aufnahme („Interview.mp3.transcribe.json“), mit Korrekturen und Clips. Wer den Ordner teilt, etwa über Nextcloud, sieht und bearbeitet dieselben Transkripte.'
							: 'Aus: Transkripte bleiben nur in Transcribe. Liegt schon eins neben einer Aufnahme, liest die App es trotzdem.'}
						onclick={() => verbundenAendern({ action: 'sidecar', on: nebenAn })}
					/>
				</div>
			</section>

			<section id="programme" class="blatt" aria-labelledby="h-programme">
				<h2 id="h-programme">Andere Programme</h2>
				<p class="hinweis">
					Programme wie Moliri können Transcribe nutzen: Sprache live als Text, Text aus Bildern lesen. Dafür muss
					Transcribe laufen, auf Wunsch auch ohne offenes Fenster als Symbol im Infobereich der Taskleiste.
				</p>
				{#if hintergrund}
					{@const h = hintergrund}
					<div class="schalter">
						{#if h.ready !== undefined}
							<Toggle
								checked={h.ready}
								title="Im Hintergrund bereit"
								description={h.ready
									? 'Programme dürfen über Transcribe das Mikrofon aufnehmen lassen, und die Erkennung bleibt geladen, damit alles sofort startet. Das belegt etwa 1 GB Arbeitsspeicher. Audiodaten und Bilder, die ein Programm schickt, verarbeitet Transcribe immer.'
									: 'Aus: Programme dürfen das Mikrofon nicht über Transcribe aufnehmen lassen, und nichts bleibt geladen. Audiodaten und Bilder, die ein Programm schickt, verarbeitet Transcribe trotzdem. Von Hand aufnehmen und diktieren geht auch, es startet nur etwas langsamer.'}
								onclick={() => hintergrundSetzen({ ready: !h.ready })}
							/>
						{/if}
						{#if h.supported}
							<Toggle
								checked={h.autostart}
								title="Mit Windows starten"
								description={h.autostart
									? 'Transcribe startet beim Anmelden ohne Fenster und wartet im Infobereich.'
									: 'Aus: Transcribe startet erst, wenn du es öffnest.'}
								onclick={() => hintergrundSetzen({ autostart: !h.autostart })}
							/>
							<Toggle
								checked={h.background || h.autostart}
								disabled={h.autostart}
								title="Nach dem Schließen weiterlaufen"
								description={h.autostart
									? 'Gilt immer, solange Transcribe mit Windows startet.'
									: h.background
										? 'Schließt du das Fenster, bleibt Transcribe im Infobereich: ein Klick darauf öffnet es, mit Rechtsklick beendest du es.'
										: 'Aus: Schließt du das Fenster, beendet sich Transcribe, sobald nichts mehr zu tun ist.'}
								onclick={() => hintergrundSetzen({ background: !h.background })}
							/>
						{/if}
					</div>
					{#if hintergrundFehler}<p class="warnung">{hintergrundFehler}</p>{/if}
				{/if}
				<a class="weiter" href="/live?verbinden" data-sveltekit-reload>
					Adresse und wer sich verbinden darf <IconMaterialSymbolsArrowForwardRounded aria-hidden="true" />
				</a>
			</section>

			{#if ocr !== false}
				<section id="ocr" class="blatt" aria-labelledby="h-ocr">
					<h2 id="h-ocr">Texterkennung</h2>
					<p class="hinweis">
						Transcribe liest auch Text aus Bildern, etwa Karteikarten, Etiketten oder Frakturdruck, für Programme wie
						Moliri. Das Bild verlässt den Rechner nicht, außer du lässt einen KI-Dienst lesen. Hier kannst du es mit einem
						eigenen Bild ausprobieren.
					</p>
					{#if !ocr}
						<p class="leer">Lade …</p>
					{:else}
						{#each ocr.engines as l (l.id)}
							<div class="leser">
								<p class="karte">
									<IconMaterialSymbolsDocumentScannerOutlineRounded aria-hidden="true" />
									<strong>{l.name}</strong>
									<span class="meta">
										{l.id === 'api' ? `Modell ${l.version}` : l.version ? `Version ${l.version}` : l.gpu ? 'auf der Grafikkarte' : ''}
									</span>
									{#if l.ready}
										<span class="zustand da"><IconMaterialSymbolsCheckRounded aria-hidden="true" />bereit</span>
									{:else if l.usable}
										<span class="zustand">lädt beim ersten Lesen</span>
									{:else}
										<span class="zustand fehlt">{l.id === 'api' ? 'nicht eingerichtet' : 'nicht installiert'}</span>
									{/if}
								</p>
								<p class="hinweis">
									{OCR_LESER[l.id] ?? ''}
									{#if l.languages.length && l.id !== 'api'}Installiert: {ocrReihe(l.languages).map(ocrName).join(', ')}.{/if}
									{#if l.download.length}
										Beim ersten Lesen geladen: {ocrReihe(l.download).map(ocrName).join(', ')}.{/if}
									{#if !l.usable && l.id === 'tesseract'}
										Unter Linux und macOS liest das installierte Programm <code>tesseract</code>; installiere es mit seinen
										Sprachen, dann lade diese Seite neu.
									{:else if !l.usable && l.error}
										{l.error}.
									{/if}
								</p>
							</div>
						{/each}

						{#if ocrDienst}
							{@const od = ocrDienst}
							{@const a = od.ocrApi}
							{@const dienst = od.ocrApiProviders.find((d) => d.id === a.provider) ?? od.ocrApiProviders[0]}
							{@const vorgabe = ocr.engines.find((l) => l.id === od.ocrEngine)}
							<div class="schalter">
								<label class="feldname">
									Für Moliri und andere Programme
									<select class="feld" value={od.ocrEngine} onchange={(ev) => ocrDienstSetzen({ ocrEngine: ev.currentTarget.value })}>
										<option value="">Automatisch</option>
										{#each ocr.engines as l (l.id)}<option value={l.id}>{l.name}</option>{/each}
									</select>
								</label>
							</div>
							<p class="hinweis">
								Damit liest Transcribe, wenn ein Programm keine Erkennung nennt, wie Moliri. Automatisch nimmt die erste,
								die liest, nie den KI-Dienst.
							</p>
							{#if vorgabe && !vorgabe.usable}
								<p class="warnung">{vorgabe.name} liest noch nicht, bis dahin wählt Transcribe automatisch.</p>
							{/if}

							<h3 class="teil">KI-Dienst</h3>
							<p class="hinweis">
								Mit deinem eigenen Schlüssel liest ein KI-Modell im Internet oder im eigenen Netz. Das ist ein anderer Dienst
								als der für die Sprache unter „Erkennung“, mit eigenem Modell und Schlüssel.
							</p>
							<div class="api">
								<div class="apifelder">
									<label class="feldname">
										Dienst
										<select class="feld" value={a.provider} onchange={(ev) => ocrApiSetzen({ provider: ev.currentTarget.value })}>
											{#each od.ocrApiProviders as d (d.id)}<option value={d.id}>{d.label}</option>{/each}
										</select>
									</label>
									<label class="feldname adresse">
										Adresse
										<input class="feld" spellcheck="false" value={a.url} onchange={(ev) => ocrApiSetzen({ url: ev.currentTarget.value })} />
									</label>
									<label class="feldname">
										Modell
										<input class="feld" spellcheck="false" value={a.model} onchange={(ev) => ocrApiSetzen({ model: ev.currentTarget.value })} />
									</label>
								</div>
								{#if dienst.kind === 'openai'}
									<p class="hinweis">
										Spricht die OpenAI-kompatible Schnittstelle, wie OpenAI, Mistral oder ein eigener Server (etwa Ollama). Das
										Modell muss Bilder lesen können, etwa qwen2.5vl.
									</p>
								{/if}

								<label class="feldname" for="ocr-schluessel">API-Schlüssel{dienst.noKey ? ' (falls der Server einen verlangt)' : ''}</label>
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
										placeholder={a.keySet ? `hinterlegt${a.keyHint ? ` (endet auf ${a.keyHint})` : ''}` : 'Schlüssel'}
										bind:value={ocrSchluessel}
									/>
									<Button type="primary" size="large" nativeType="submit" disabled={!ocrSchluessel.trim()} loading={speichertOcrSchluessel}>
										{a.keySet ? 'Ersetzen' : 'Speichern'}
									</Button>
									{#if a.keySet}
										<Button type="secondary" size="large" onclick={() => ocrSchluesselSpeichern('')}>Entfernen</Button>
									{/if}
								</form>
								{#if ocrSchluesselStand}
									<p class="gespeichert" role="status">{ocrSchluesselStand}</p>
								{:else if !a.keySet && !dienst.noKey}
									<p class="warnung">Ohne Schlüssel kann die App {dienst.label} nicht nutzen.</p>
								{/if}
								<p class="hinweis">
									{#if dienst.keys}
										Den Schlüssel legst du in deinem Konto bei
										<a href={dienst.keys} target="_blank" rel="noreferrer">{dienst.label}</a> an. Abgerechnet wird nach
										Tokens, für jedes Bild und den Text, den das Modell schreibt.
									{/if}
									Der Schlüssel bleibt in den Einstellungen auf diesem Rechner; Moliri bekommt ihn nicht.
								</p>
							</div>
							{#if ocrDienstFehler}<p class="warnung">{ocrDienstFehler}</p>{/if}
						{/if}

						{@const lesbar = ocrLesbar()}
						{@const leser = lesbar.find((l) => l.id === ocrLeser) ?? lesbar[0]}
						{#if leser}
							{@const sprachen = ocrReihe([...leser.languages, ...leser.download])}
							<div class="probe">
								<div class="apifelder">
									{#if lesbar.length > 1}
										<label class="feldname">
											Erkennung
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
										Sprache und Schrift
										<select class="feld" bind:value={ocrSprachen}>
											{#each sprachen as s (s)}<option value={s}>{ocrName(s)}</option>{/each}
											{#if sprachen.includes('deu') && sprachen.includes('frak2021')}
												<option value="deu+frak2021">Deutsch und Fraktur</option>
											{/if}
											{#if sprachen.includes('deu') && sprachen.includes('eng')}
												<option value="deu+eng">Deutsch und Englisch</option>
											{/if}
										</select>
									</label>
									{#if leser.layouts.length > 1}
										<label class="feldname">
											Was auf dem Bild ist
											<select class="feld" bind:value={ocrLayout}>
												{#each leser.layouts as la (la)}<option value={la}>{OCR_LAYOUTS[la] ?? la}</option>{/each}
											</select>
										</label>
									{/if}
								</div>
								<label class="bildwahl" class:liest>
									{#if liest}<IconEosIconsLoading aria-hidden="true" />{:else}<IconMaterialSymbolsImageOutlineRounded aria-hidden="true" />{/if}
									{liest ? 'Liest …' : 'Bild lesen'}
									<input class="sr" type="file" accept="image/*" disabled={liest} onchange={bildLesen} />
								</label>
								{#if liest && leser.id === 'api'}
									<p class="hinweis">Das Bild ist beim KI-Dienst. Große Modelle denken bei einer vollen Seite eine Weile nach.</p>
								{:else if liest && !leser.ready}
									<p class="hinweis">
										{leser.id === 'onnx'
											? 'Beim ersten Mal lädt Transcribe die Modelle von PP-OCRv5 herunter (13 MB), das dauert etwas.'
											: 'Beim ersten Mal lädt Transcribe Tesseract und die Sprache herunter, das dauert etwas.'}
									</p>
								{/if}
								{#if ocrFehler}<p class="warnung">{ocrFehler}</p>{/if}
								{#if gelesen}
									<div class="ergebnis">
										<svg
											class="seite"
											viewBox="0 0 {gelesen.breite} {gelesen.hoehe}"
											role="img"
											aria-label="Das gelesene Bild, die gefundenen Wörter umrahmt"
										>
											<image href={gelesen.bild} width={gelesen.breite} height={gelesen.hoehe} />
											{#each gelesen.kaesten as k, i (i)}<rect x={k[0]} y={k[1]} width={k[2]} height={k[3]} />{/each}
										</svg>
										<div class="gelesen">
											<p class="meta">
												{gelesen.zeilen === 1 ? '1 Zeile' : `${gelesen.zeilen} Zeilen`} in {gelesen.sekunden
													.toFixed(1)
													.replace('.', ',')} s{gelesen.leser ? `, ${gelesen.leser}` : ''}
											</p>
											<pre>{gelesen.text || 'Kein Text gefunden.'}</pre>
										</div>
									</div>
								{/if}
							</div>
						{/if}
					{/if}
				</section>
			{/if}

			{#if diktat?.supported}
				{@const dk = diktat}
				{@const kuerzel = dk.keys.find((k) => k.key === dk.key)?.label}
				<section id="diktieren" class="blatt" aria-labelledby="h-diktat">
					<h2 id="h-diktat">Diktieren in jedem Programm</h2>
					<p class="hinweis">
						{#if kuerzel}
							Drücke <kbd>{kuerzel}</kbd> in einem beliebigen Programm und sprich, dann drücke es noch einmal.
							<kbd>Esc</kbd> bricht ab. Ein kleines Fenster über der Taskleiste zeigt, dass Transcribe zuhört und was
							es verstanden hat, wie ein Untertitel. Ein Klick darauf öffnet das Diktat auf der Seite Live.
						{:else}
							Mit einem Tastenkürzel diktierst du in jedes Programm: Text, der dort erscheint, wo der Cursor steht.
						{/if}
					</p>
					<div class="schalter">
						<label class="feldname">
							Tastenkürzel
							<select class="feld" value={dk.key} onchange={(e) => diktatSetzen({ key: e.currentTarget.value })}>
								{#each dk.keys as k (k.key)}<option value={k.key}>{k.label}</option>{/each}
								<option value="off">Aus</option>
							</select>
						</label>
						<Toggle
							checked={dk.insert}
							disabled={dk.key === 'off'}
							title="Am Cursor einfügen"
							description={dk.insert ? 'Der Text erscheint, wo der Cursor steht.' : 'Aus: Der Text wird nirgends eingefügt.'}
							onclick={() => diktatSetzen({ insert: !dk.insert })}
						/>
						<Toggle
							checked={dk.clipboard}
							disabled={dk.key === 'off'}
							title="In die Zwischenablage"
							description={dk.clipboard
								? 'Der Text liegt danach in der Zwischenablage, für Strg + V.'
								: 'Aus: Die Zwischenablage bleibt, wie sie war.'}
							onclick={() => diktatSetzen({ clipboard: !dk.clipboard })}
						/>
					</div>
					{#if dk.key !== 'off' && !dk.insert && !dk.clipboard}
						<p class="hinweis">
							Beides aus: Das Diktat erscheint nur im kleinen Fenster und bleibt auf der Seite Live stehen, als Notiz
							oder zum Kopieren von Hand.
						</p>
					{/if}
					<p class="hinweis">
						Diktiert wird mit dem Mikrofon und der Sprache, die auf der Seite <a href="/live">Live</a> eingestellt sind.
						Jedes Diktat steht danach auch dort.
					</p>
					{#if dk.error}<p class="warnung">{dk.error}</p>{/if}
					{#if diktatFehler}<p class="warnung">{diktatFehler}</p>{/if}
				</section>
			{/if}

			<section id="gpu" class="blatt" aria-labelledby="h-gpu">
				<h2 id="h-gpu">Grafikkarte</h2>
				{#if !gpu}
					<p class="leer">Prüfe die Grafikkarte …</p>
				{:else if !gpu.gpu.present}
					<p>Keine NVIDIA-GPU erkannt. Die Erkennung läuft auf dem Prozessor (CPU).</p>
					<p class="hinweis">GPU-Beschleunigung gibt es nur mit NVIDIA-Grafikkarten (CUDA) unter Windows und Linux.</p>
				{:else}
					{@const g = gpu.gpu}
					{@const laedt = imGang(nvidia) ? nvidia : imGang(cuda) ? cuda : null}
					<p class="karte">
						<IconMaterialSymbolsMemoryOutlineRounded aria-hidden="true" />
						<strong>{g.name}</strong>
						<span class="meta">{Math.round(g.memoryMB / 1024)} GB · Treiber {g.driver}</span>
					</p>
					{#if !g.supported}
						<p class="hinweis">Für diese Grafikkarte oder dieses System gibt es keine passende GPU-Runtime.</p>
					{:else if laedt}
						{@const was = laedt === nvidia ? 'Bibliotheken von NVIDIA werden geladen' : 'GPU-Runtime wird geladen'}
						<p>{was}{laedt.total ? ` – ${laedt.done ? groesse(laedt.done) : '0 MB'} von ${groesse(laedt.total)}` : ' …'}</p>
						<div class="balken" role="progressbar" aria-label={was} aria-valuenow={prozent(laedt)} aria-valuemin="0" aria-valuemax="100">
							<i style:width="{prozent(laedt)}%"></i>
						</div>
						{#if laedt === nvidia && imGang(cuda)}<p class="hinweis">Danach folgt die GPU-Runtime.</p>{/if}
					{:else if !g.ready}
						{@const toolkit = g.cudaMajor === 13 ? 'https://developer.nvidia.com/cuda-downloads' : 'https://developer.nvidia.com/cuda-toolkit-archive'}
						<p class="warnung">Für die GPU fehlen Bibliotheken von NVIDIA: {g.missing.join(', ')}.</p>
						{#if g.fetch}
							<p class="hinweis">
								Die App kann sie selbst bei NVIDIA laden, etwa {groesse(g.fetch)}{#if !gpu.installed}, dazu die GPU-Runtime
									mit etwa 500 MB{/if}. Das geschieht einmal. Mit „{gpu.installed ? 'Bibliotheken laden' : 'GPU einrichten'}“
								stimmst du den Lizenzbedingungen von NVIDIA zu, für das
								<a href="https://docs.nvidia.com/cuda/eula/" target="_blank" rel="noreferrer">CUDA Toolkit</a> und für
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
									{gpu.installed ? 'Bibliotheken laden' : 'GPU einrichten'}
								</Button>
								<Button type="secondary" size="large" loading={pruefe} onclick={erneutPruefen}>
									{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
									Erneut prüfen
								</Button>
							</div>
							{#if fehler.nvidia || gpuFehler}
								<p class="warnung">{fehler.nvidia || gpuFehler}</p>
							{:else if nvidia?.state === 'error'}
								<div>
									<p class="warnung">Download fehlgeschlagen. Ist der Rechner mit dem Internet verbunden?</p>
									<details>
										<summary>Einzelheiten</summary>
										<p class="technik">{nvidia.error}</p>
									</details>
								</div>
							{/if}
							<p class="hinweis">
								Lieber selbst installieren? Das
								<a href={toolkit} target="_blank" rel="noreferrer">CUDA Toolkit {g.cudaMajor}</a> und
								<a href="https://developer.nvidia.com/cudnn-downloads" target="_blank" rel="noreferrer">cuDNN 9</a> gibt es
								bei NVIDIA, danach hier „Erneut prüfen“.
							</p>
						{:else}
							<p class="hinweis">
								Installiere das
								<a href={toolkit} target="_blank" rel="noreferrer">CUDA Toolkit {g.cudaMajor}</a> und
								<a href="https://developer.nvidia.com/cudnn-downloads" target="_blank" rel="noreferrer">cuDNN 9</a> von
								NVIDIA, dann prüfe hier erneut.
							</p>
							<div class="handeln">
								<Button type="secondary" size="large" loading={pruefe} onclick={erneutPruefen}>
									{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
									Erneut prüfen
								</Button>
							</div>
						{/if}
					{:else if !gpu.installed}
						<p class="hinweis">
							Die GPU-Runtime (CUDA {g.cudaMajor}) ist etwa 500 MB groß und wird einmal geladen. Ohne sie läuft alles auf
							dem Prozessor.
						</p>
						<div class="handeln">
							<Button type="primary" size="large" onclick={gpuLaden}>
								{#snippet iconLeft()}<IconMaterialSymbolsDownloadRounded />{/snippet}
								GPU-Runtime herunterladen
							</Button>
						</div>
						{#if cuda?.state === 'error'}<p class="warnung">{cuda.error}</p>{/if}
						{#if gpuFehler}<p class="warnung">{gpuFehler}</p>{/if}
					{:else}
						<Toggle
							checked={gpu.accel === 'cuda'}
							title="Mit der GPU rechnen (CUDA)"
							description="Aus: Die Erkennung läuft auf dem Prozessor."
							onclick={() => mitGpu(gpu?.accel !== 'cuda')}
						/>
						<p>
							Gerade in Benutzung: <strong>{gpu.active === 'cuda' ? 'GPU' : 'Prozessor (CPU)'}</strong>.
							{#if gpu.restartNeeded}<span class="warnung">Die Änderung wirkt, wenn du die App neu startest.</span>{/if}
						</p>
					{/if}
				{/if}
			</section>

			<section id="speicher" class="blatt" aria-labelledby="h-speicher">
				<h2 id="h-speicher">Speicher und Modelle</h2>
				<p class="hinweis">
					Alles liegt im Datenordner: Aufnahmen, Ergebnisse, Modelle und Einstellungen. Zum Sichern genügt es, diesen Ordner
					zu kopieren. Wo die EXE liegt, spielt keine Rolle.
				</p>
				{#if ort}
					<h3 class="teil">Ort des Datenordners</h3>
					<p class="ortzeile">
						<span class="pfad">{ort.dataDir}</span>
						{#if amStandard}<span class="zustand da">Standard</span>{/if}
					</p>
					{#if ort.fixed}
						<p class="hinweis">
							Beim Start mit <code>--data</code> oder <code>TRANSCRIBE_DATA</code> festgelegt; deshalb lässt er sich hier nicht
							ändern.
						</p>
					{:else if umzug?.state === 'restarting' || (ortUnterwegs && (offline || !umzug))}
						<!-- kopiert ist es oft zwischen zwei Abfragen, dann ist die App schon beim Neustart -->
						<p role="status">Die Daten sind umgezogen. Transcribe startet neu …</p>
					{:else if umzug?.state === 'copying'}
						<div class="umzug" role="status">
							<p>Die Daten ziehen nach <span class="pfad">{umzug.target}</span> um: {groesse(umzug.done)} von {groesse(umzug.total)}</p>
							<div
								class="balken"
								role="progressbar"
								aria-label="Umzug des Datenordners"
								aria-valuenow={umzug.total ? Math.round((umzug.done * 100) / umzug.total) : 0}
								aria-valuemin="0"
								aria-valuemax="100"
							>
								<i style:width="{umzug.total ? Math.round((umzug.done * 100) / umzug.total) : 0}%"></i>
							</div>
						</div>
					{:else if ortZiel}
						<div class="umzug">
							<h4>Datenordner umziehen?</h4>
							<p>
								Die Daten ({groesse(ortZiel.size)}) ziehen nach <span class="pfad">{ortZiel.target}</span>. Danach startet
								Transcribe neu und löscht den alten Ordner.
							</p>
							<div class="handeln">
								<Button type="primary" size="large" loading={ortLaeuft === 'umziehen'} onclick={umziehen}>
									{#snippet iconLeft()}<IconMaterialSymbolsDriveFileMoveOutlineRounded />{/snippet}
									Umziehen
								</Button>
								<Button type="secondary" size="large" onclick={() => (ortZiel = null)}>Abbrechen</Button>
							</div>
						</div>
					{:else}
						{#if !ort.pick}
							<div class="schluesselzeile">
								<input class="feld" spellcheck="false" placeholder="Neuer Ort, z. B. /mnt/daten/transcribe" bind:value={ortEingabe} aria-label="Neuer Ort des Datenordners" />
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
								Anderen Ort wählen …
							</Button>
							{#if !amStandard}
								<Button type="secondary" size="large" loading={ortLaeuft === 'standard'} onclick={() => ortPruefen('')}>
									Zurück an den Standardort
								</Button>
							{/if}
						</div>
						{#if ortFertig}<p role="status">Die Daten liegen jetzt in <span class="pfad">{ortFertig}</span>.</p>{/if}
					{/if}
					{#if umzug?.state === 'error'}<p class="warnung">{umzug.error}</p>{/if}
					{#if ortFehler}<p class="warnung">{ortFehler}</p>{/if}
				{/if}
				<h3 class="teil">Speicherorte</h3>
				{#if offline && !ortUnterwegs}
					<p class="leer fehler">Keine Verbindung zu Transcribe. Läuft das Programm noch?</p>
				{:else if !stand}
					<p class="leer">Lade …</p>
				{:else}
					<ul class="liste" aria-label="Speicherorte">
						{#each stand.folders as f (f.key)}
							{@const name = ORDNER[f.key] || f.key}
							<li class="eintrag">
								<div class="was">
									<span class="name">{name}</span>
									<span class="pfad">{f.path}</span>
								</div>
								<span class="groesse">{groesse(f.size)}</span>
								<div class="tun">
									<Button type="secondary" size="large" aria-label="{name} öffnen" onclick={() => oeffnen(f.key)}>
										{#snippet iconLeft()}<IconMaterialSymbolsFolderOpenOutlineRounded />{/snippet}
										Öffnen
									</Button>
								</div>
								{#if fehler[f.key]}<p class="fehler voll">{fehler[f.key]}</p>{/if}
							</li>
						{/each}
					</ul>
				{/if}

				<h3 class="teil">Modelle und Komponenten</h3>
				<p class="hinweis">
					{#if erkennung?.recognition === 'api'}
						Die Erkennung läuft über die API, deshalb lädt die App keine Modelle, nur ffmpeg für Wellenform und Export.
						Was schon da ist, bleibt liegen.
					{:else}
						Die App lädt sie beim ersten Start. Was „bei Bedarf“ heißt, kommt erst dazu, wenn es gebraucht wird, etwa Whisper
						turbo für Fremdsprachen.
					{/if}
				</p>
				{#if stand}
					<ul class="liste" aria-label="Modelle und Komponenten">
						{#each komponenten as c (c.name)}
							{@const laedt = c.state === 'downloading'}
							<li class="eintrag">
								<div class="was">
									<span class="name">{c.label || c.name}</span>
									{#if c.device}<span class="geraet">läuft auf der Grafikkarte: {c.device}</span>{/if}
									<span class="pfad">{c.path}</span>
								</div>
								<span class="groesse">{laedt && c.total ? `${groesse(c.done)} / ${groesse(c.total)}` : groesse(c.size)}</span>
								<div class="tun">
									{#if laedt}
										<span class="zustand laedt">lädt {c.total ? `${prozent(c)} %` : '…'}</span>
									{:else if c.state === 'error'}
										<span class="zustand fehlt">Fehler</span>
									{:else if c.installed}
										<span class="zustand da"><IconMaterialSymbolsCheckRounded aria-hidden="true" />installiert</span>
									{:else}
										<span class="zustand" class:fehlt={!c.onDemand}>{c.onDemand ? 'bei Bedarf' : 'fehlt'}</span>
									{/if}
									{#if !c.installed && !laedt}
										<Button
											type="secondary"
											size="large"
											loading={fordert.includes(c.name)}
											aria-label="{c.label || c.name} {c.state === 'error' ? 'erneut laden' : 'herunterladen'}"
											onclick={() => herunterladen(c.name)}
										>
											{#snippet iconLeft()}<IconMaterialSymbolsDownloadRounded />{/snippet}
											{c.state === 'error' ? 'Erneut laden' : 'Herunterladen'}
										</Button>
									{/if}
								</div>
								{#if laedt && c.total}
									<div class="balken voll" role="progressbar" aria-label="Download {c.label || c.name}" aria-valuenow={prozent(c)} aria-valuemin="0" aria-valuemax="100">
										<i style:width="{prozent(c)}%"></i>
									</div>
								{/if}
								{#if fehler[c.name]}
									<p class="fehler voll">{fehler[c.name]}</p>
								{:else if c.error}
									<!-- die Meldung des Servers ist technisch (URL, Proxy); zuerst ein Satz, der weiterhilft -->
									<div class="voll">
										<p class="fehler">Download fehlgeschlagen. Ist der Rechner mit dem Internet verbunden?</p>
										<details>
											<summary>Einzelheiten</summary>
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
				<h2 id="h-programm">Über Transcribe</h2>
				{#if update.stand}
					{@const u = update.stand}
					{@const neu = versionText(u.latest)}
					<p class="karte">
						<strong>Version {versionText(u.version)}</strong>
						{#if u.checks && u.state === 'current'}
							<span class="meta">die neueste{u.checked ? `, zuletzt nachgesehen ${wann(u.checked)}` : ''}</span>
						{/if}
					</p>
					{#if vorher}
						<p class="gespeichert" role="status">Aktualisiert von Version {versionText(vorher)} auf {versionText(u.version)}.</p>
					{/if}
					{#if u.manual}
						<p class="hinweis">
							Unter Linux und macOS aktualisiert sich Transcribe nicht selbst. Neue Versionen gibt es unter
							<a href="https://github.com/moliri-app/transcribe/releases" target="_blank" rel="noreferrer">Releases</a>.
						</p>
					{:else if !u.checks}
						<p class="hinweis">
							Eine Entwicklerversion sucht nicht nach Updates. Fertige Versionen gibt es unter
							<a href="https://github.com/moliri-app/transcribe/releases" target="_blank" rel="noreferrer">Releases</a>.
						</p>
					{:else if u.state === 'available' || u.state === 'failed'}
						<div class="update" role="group" aria-labelledby="h-update">
							<h3 id="h-update">Version {neu} ist da</h3>
							<p class="hinweis">
								Transcribe lädt sie herunter, tauscht das Programm aus und startet neu. Laufende Aufträge und
								Aufnahmen laufen vorher zu Ende, deine Aufnahmen und Einstellungen bleiben, wie sie sind.
								{#if u.page}<a href={u.page} target="_blank" rel="noreferrer">Was ist neu?</a>{/if}
							</p>
							<div class="handeln">
								<Button type="primary" size="large" loading={updateLaeuft === 'install'} onclick={() => updateTun('install')}>
									{#snippet iconLeft()}<IconMaterialSymbolsUpgradeRounded />{/snippet}
									{u.state === 'failed' ? 'Erneut versuchen' : 'Jetzt aktualisieren'}
								</Button>
							</div>
							{#if u.state === 'failed'}<p class="warnung">{u.error}</p>{/if}
						</div>
					{:else if u.state === 'downloading'}
						<p>Version {neu} wird geladen …</p>
						<div
							class="balken"
							role="progressbar"
							aria-label="Version {neu} wird geladen"
							aria-valuenow={Math.round((u.progress ?? 0) * 100)}
							aria-valuemin="0"
							aria-valuemax="100"
						>
							<i style:width="{Math.round((u.progress ?? 0) * 100)}%"></i>
						</div>
					{:else if u.state === 'waiting'}
						<p role="status">
							Version {neu} ist installiert. Transcribe startet neu, sobald laufende Aufträge und Aufnahmen fertig
							sind.
						</p>
					{:else if u.state === 'restarting' || update.weg}
						<p role="status">Transcribe startet neu …</p>
					{:else}
						<div class="handeln">
							<Button type="secondary" size="large" loading={updateLaeuft === 'check'} onclick={() => updateTun('check')}>
								{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
								Nach Updates suchen
							</Button>
						</div>
					{/if}
					{#if updateFehler}<p class="warnung">{updateFehler}</p>{/if}
				{/if}
				{#if stand}
					<dl class="angaben">
						<dt>Programm</dt>
						<dd class="pfad">{stand.exe}</dd>
						<dt>Protokoll</dt>
						<dd class="pfad">{stand.log}</dd>
						<dt>System</dt>
						<dd>{stand.os}</dd>
						<dt>sherpa-onnx</dt>
						<dd>{stand.sherpa}</dd>
					</dl>
				{/if}
				<a class="weiter" href="/lizenzen">
					Lizenzen und Drittkomponenten <IconMaterialSymbolsArrowForwardRounded aria-hidden="true" />
				</a>
			</section>
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
	/* wo man ist: Tintenpille wie der aktive Tab der App-Leiste, umgekehrt */
	.menue a[aria-current='location'] {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
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
