<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Bibliothek, die Startseite: die transkribierten Aufnahmen als Blätter, jedes mit dem Anfang
     seines Transkripts wie im Manuskript, wer spricht und wie viel schon von Hand korrigiert ist;
     ein Klick öffnet es im Editor. Darüber, nur wenn es sie gibt, was gerade transkribiert wird
     oder gescheitert ist, und die Aufnahmen, die noch kein Transkript haben, zum Einreihen.
     Steht unter / und, für alte Lesezeichen, unter /queue. -->
<script>
	import { onMount, untrack } from 'svelte';
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import AuftragsOptionen from '$lib/AuftragsOptionen.svelte';
	import { auftraege, geaendert } from '$lib/auftraege.svelte.js';
	import { dauer, einreihen, getJSON, post, upload, vorgaben } from '$lib/api.js';
	import { sprachname } from '$lib/live/stand.svelte.js';

	/** @typedef {{ id: number, name: string }} Sprecher */
	/** @typedef {{ duration: number, speakers: Sprecher[], preview: string, edits: number, clips: number, done: number }} Inhalt */
	/** @typedef {{ name: string, stem: string, video: boolean, added: number, summary?: Inhalt }} Aufnahme */
	/** @typedef {{ id: string, audio: string, stem: string, status: string, progress: number, step: string | null,
	 *   log: string[] | null, started: number | null, finished: number | null, error: string | null, note: string | null,
	 *   options: { preset: string, speakers: number | null, language?: string, engine?: string } }} Auftrag */

	const STATUS = /** @type {Record<string, string>} */ ({
		queued: 'wartet',
		running: 'läuft',
		canceling: 'bricht ab',
		error: 'Fehler'
	});
	const LAUFEND = ['queued', 'running', 'canceling'];

	const SORTEN = /** @type {const} */ ([
		['zuletzt', 'Zuletzt bearbeitet'],
		['name', 'Name'],
		['laenge', 'Länge']
	]);

	/** @type {Aufnahme[] | null} */
	let aufnahmen = $state(null);
	/** @type {Auftrag[] | null} */
	let jobs = $state(null);
	let offline = $state(false);
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
	/** Uhr für die Laufzeit laufender Aufträge, in Sekunden */
	let jetzt = $state(Date.now() / 1000);
	/** die Aufnahme, nach deren Löschen gerade gefragt wird */
	let loeschen = $state(/** @type {Aufnahme | null} */ (null));
	let loescht = $state(false);
	/** @type {HTMLDialogElement} */
	let rueckfrage;

	/** je Aufnahme ihr jüngster Auftrag (die Warteschlange führt sie in der Reihenfolge des Einreihens) */
	const letzter = $derived(new Map((jobs ?? []).map((j) => [j.audio, j])));
	/** was gerade transkribiert wird oder zuletzt gescheitert ist */
	const inArbeit = $derived([...letzter.values()].filter((j) => LAUFEND.includes(j.status) || j.status === 'error'));
	const laufend = $derived(inArbeit.filter((j) => LAUFEND.includes(j.status)));
	/** Aufnahmen ohne Transkript, die auch nicht gerade in Arbeit sind */
	const neu = $derived((aufnahmen ?? []).filter((a) => !a.summary && !inArbeit.some((j) => j.audio === a.name)));
	const fertig = $derived((aufnahmen ?? []).filter((a) => a.summary));

	const regal = $derived.by(() => {
		const q = suche.trim().toLowerCase();
		const treffer = q
			? fertig.filter((a) =>
					[a.name, a.summary?.preview ?? '', ...(a.summary?.speakers ?? []).map((s) => s.name)].some((t) =>
						t.toLowerCase().includes(q)
					)
				)
			: fertig;
		const nach = {
			zuletzt: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => (b.summary?.done ?? 0) - (a.summary?.done ?? 0),
			name: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => a.name.localeCompare(b.name, 'de'),
			laenge: (/** @type {Aufnahme} */ a, /** @type {Aufnahme} */ b) => (b.summary?.duration ?? 0) - (a.summary?.duration ?? 0)
		}[sorte];
		return treffer.toSorted(nach);
	});

	/** der Name ohne Endung */
	const titel = (/** @type {string} */ name) => name.replace(/\.[^.]+$/, '');

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

	async function ladeAufnahmen() {
		try {
			aufnahmen = await getJSON('/api/index');
		} catch {
			aufnahmen ??= [];
		}
	}

	async function ladeAuftraege() {
		const vorher = laufend.length;
		try {
			jobs = (await getJSON('/api/queue')).jobs;
			offline = false;
		} catch {
			offline = true;
			return;
		}
		// ein Auftrag ist fertig geworden: sein Transkript steht jetzt im Regal
		if (laufend.length < vorher) ladeAufnahmen();
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
			meldung = n ? `„${titel(a.name)}“ wird neu transkribiert. Deine Korrekturen bleiben erhalten.` : 'Schon in Arbeit.';
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
			// das Hochgeladene ist gleich angekreuzt, ein Klick auf „Transkribieren“ genügt
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

	/** @param {Aufnahme} a */
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
			meldung = `„${titel(a.name)}“ ist gelöscht.`;
			gewaehlt = gewaehlt.filter((n) => n !== a.name);
		} catch (e) {
			meldung = /** @type {Error} */ (e).message;
		} finally {
			loescht = false;
			rueckfrage.close();
		}
		ladeAufnahmen();
		geaendert();
	}

	/** Der Editor öffnet die Aufnahme, die hier zuletzt gewählt wurde. */
	function imEditor(/** @type {string} */ stem) {
		try {
			localStorage.setItem('ed:stem', stem);
		} catch {
			// ohne Speicher öffnet der Editor die zuletzt dort gewählte Aufnahme
		}
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

	/** @param {Auftrag} j */
	function laufzeit(j) {
		if (!j.started) return '';
		return dauer((j.finished || jetzt) - j.started);
	}

	// Nach dem Ablegen von Dateien im Fenster (Einrichtung.svelte) oder einer eigenen Aktion sofort neu laden
	// (nur auf auftraege.stand hören: ladeAuftraege liest die Aufträge, die es selbst setzt, und lüde sonst endlos neu)
	$effect(() => {
		auftraege.stand;
		untrack(() => {
			ladeAufnahmen();
			ladeAuftraege();
		});
	});

	onMount(() => {
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
		return () => clearInterval(t);
	});
</script>

<svelte:head><title>Bibliothek · Transcribe by Moliri</title></svelte:head>

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

	<p class="meldung" role="status">{meldung}</p>

	{#if offline}
		<p class="leer fehler">Server nicht erreichbar. Läuft das Programm noch?</p>
	{/if}

	{#if inArbeit.length}
		<section class="blatt" aria-labelledby="h-arbeit">
			<div class="zeile">
				<h2 id="h-arbeit">In Arbeit</h2>
				{#if laufend.length}<span class="zaehler">{laufend.length}</span>{/if}
				<span class="platz"></span>
				{#if laufend.length > 1}
					<Button type="secondary" size="large" onclick={alleAbbrechen}>Alle abbrechen</Button>
				{/if}
			</div>
			<ul class="auftraege" aria-label="Aufträge">
				{#each inArbeit as j (j.id)}
					{@const pct = Math.round((j.progress || 0) * 100)}
					{@const laeuft = LAUFEND.includes(j.status)}
					<li class="auftrag">
						<div class="kopfzeile">
							<div class="was">
								<span class="name">{titel(j.audio)}</span>
								<span class="status {j.status}">{STATUS[j.status] || j.status}</span>
								<span class="meta">{wie(j)}{#if j.started}&nbsp;· <span class="zeit">{laufzeit(j)}</span>{/if}</span>
							</div>
							<div class="tun">
								{#if laeuft}
									<Button type="secondary" size="large" disabled={j.status === 'canceling'} onclick={() => aktion(j, 'cancel')}>Abbrechen</Button>
								{:else}
									<Button type="secondary" size="large" onclick={() => aktion(j, 'retry')}>
										{#snippet iconLeft()}<IconMaterialSymbolsRefreshRounded />{/snippet}
										Erneut
									</Button>
									<Button type="tertiary" size="large" onclick={() => aktion(j, 'remove')}>Entfernen</Button>
								{/if}
							</div>
						</div>
						{#if j.status === 'running'}
							<div class="balken" role="progressbar" aria-label="Fortschritt {j.audio}" aria-valuenow={pct} aria-valuemin="0" aria-valuemax="100">
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
					<div class="aufnahme" class:gewaehlt={gewaehlt.includes(a.name)}>
						<label>
							<input type="checkbox" value={a.name} bind:group={gewaehlt} />
							<span class="name">{a.name}</span>
							{#if a.video}
								<span class="art" title="Video"><IconMaterialSymbolsMovieOutlineRounded aria-hidden="true" /><span class="sr">Video</span></span>
							{/if}
							<span class="hinzu">{wann(a.added)}</span>
						</label>
						<button class="weg" type="button" title="Löschen" aria-label="{titel(a.name)} löschen" onclick={() => fragLoeschen(a)}>
							<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />
						</button>
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
			<label class="willkommen">
				<IconMaterialSymbolsLibraryMusicOutlineRounded aria-hidden="true" />
				<strong>Noch keine Aufnahmen</strong>
				<span>Zieh Audio- oder Videodateien in dieses Fenster, oder wähle sie hier aus. Transcribe schreibt mit, wer was sagt.</span>
				<span class="knopf">Dateien auswählen</span>
				<input class="sr" type="file" accept="audio/*,video/*" multiple onchange={hochladen} />
			</label>
		{:else if !fertig.length}
			<p class="leer">Noch kein Transkript fertig. Was du oben transkribierst, steht danach hier.</p>
		{:else if !regal.length}
			<p class="leer">Nichts gefunden zu „{suche}“.</p>
		{:else}
			<ul class="karten" aria-label="Transkribierte Aufnahmen">
				{#each regal as a (a.name)}
					{@const s = /** @type {Inhalt} */ (a.summary)}
					{@const j = letzter.get(a.name)}
					{@const arbeitet = !!j && LAUFEND.includes(j.status)}
					{@const erster = s.speakers[0]}
					<li class="karte" class:arbeitet>
						<h3 class="titel">
							<a href="/editor" data-sveltekit-reload onclick={() => imEditor(a.stem)} title="{a.name} im Editor öffnen">{titel(a.name)}</a>
						</h3>
						<p class="stand">
							{#if a.video}<IconMaterialSymbolsMovieOutlineRounded aria-label="Video" />{/if}
							<span class="zeit">{dauer(s.duration)}</span>
							<span aria-hidden="true">·</span>
							<span>{s.speakers.length === 1 ? '1 Sprecher' : `${s.speakers.length} Sprecher`}</span>
							<span aria-hidden="true">·</span>
							<span>{wann(s.done)}</span>
							{#if arbeitet}
								<span class="status running">wird neu transkribiert</span>
							{:else if frisch(a)}
								<span class="status done">neu</span>
							{/if}
						</p>
						<p class="auszug">
							{#if erster}<span class="wer" style:--c="var(--spk{erster.id % 8})">{erster.name}</span>{/if}
							{s.preview || 'Kein Text erkannt.'}
						</p>
						<div class="fuss">
							<ul class="stimmen" aria-label="Sprecher">
								{#each s.speakers.slice(0, 4) as p (p.id)}
									<li><span class="punkt" style:--c="var(--spk{p.id % 8})" aria-hidden="true"></span>{p.name}</li>
								{/each}
								{#if s.speakers.length > 4}<li class="mehr">+{s.speakers.length - 4}</li>{/if}
							</ul>
							{#if s.edits || s.clips}
								<p class="arbeit">
									{#if s.edits}<span title="Von Hand korrigiert"><IconMaterialSymbolsEditOutlineRounded aria-hidden="true" />{s.edits === 1 ? '1 Korrektur' : `${s.edits} Korrekturen`}</span>{/if}
									{#if s.clips}<span title="Clips"><IconMaterialSymbolsContentCutRounded aria-hidden="true" />{s.clips === 1 ? '1 Clip' : `${s.clips} Clips`}</span>{/if}
								</p>
							{/if}
							<button
								class="neu-tx"
								type="button"
								disabled={arbeitet}
								title="Neu transkribieren; Korrekturen von Hand bleiben erhalten"
								aria-label="{titel(a.name)} neu transkribieren"
								onclick={() => neuTranskribieren(a)}
							>
								<IconMaterialSymbolsRefreshRounded aria-hidden="true" />
							</button>
							<button
								class="weg"
								type="button"
								disabled={inBearbeitung(a)}
								title={inBearbeitung(a) ? 'Wird gerade transkribiert; erst abbrechen, dann löschen' : 'Löschen'}
								aria-label="{titel(a.name)} löschen"
								onclick={() => fragLoeschen(a)}
							>
								<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />
							</button>
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<dialog class="rueckfrage" bind:this={rueckfrage} aria-labelledby="h-loeschen" onclose={() => (loeschen = null)}>
		{#if loeschen}
			{@const s = loeschen.summary}
			<h2 id="h-loeschen">„{titel(loeschen.name)}“ löschen?</h2>
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
		max-width: 76rem;
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
	.aufnahme label {
		flex: 1;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		min-height: var(--hit);
		padding: 0.25rem 0 0.25rem 0.75rem;
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
		overflow-wrap: anywhere;
	}
	.art {
		display: inline-flex;
		color: hsl(var(--color-text-secondary));
	}
	.hinzu {
		margin-left: auto;
		padding-left: 1rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-tertiary));
		white-space: nowrap;
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

	.karten {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(min(100%, 21rem), 1fr));
		gap: 1.25rem;
	}

	/* Ein Blatt je Aufnahme: oben der Titel, darunter der Anfang des Transkripts wie im Manuskript,
	   mit dem ersten Sprecher in Versalien und seiner Farbe. Der Titel ist der Link; er deckt das
	   ganze Blatt ab, nur „Neu transkribieren“ liegt darüber. */
	.karte {
		position: relative;
		display: grid;
		grid-template-rows: auto auto 1fr auto;
		gap: 0.5rem;
		padding: 1.25rem 1.375rem 1rem;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		box-shadow: 0 1px 2px hsl(var(--color-shadow)), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
		transition: box-shadow 0.2s ease-out, transform 0.2s ease-out;
	}
	.karte:hover {
		box-shadow: 0 1px 2px hsl(var(--color-shadow)), 0 18px 36px -18px hsl(var(--color-neutral-100) / 0.38);
		transform: translateY(-2px);
	}
	.karte:has(.titel a:focus-visible) {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.karte.arbeitet {
		box-shadow: inset 0 0 0 2px hsl(var(--color-green-100)), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.25);
	}
	.titel {
		padding-right: 2.5rem;
		font-size: 1.1875rem;
		letter-spacing: -0.005em;
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
		border-radius: inherit;
	}
	.karte:hover .titel a {
		text-decoration: underline;
		text-decoration-color: hsl(var(--color-green-100));
		text-decoration-thickness: 2px;
		text-underline-offset: 0.2em;
	}
	.stand {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 0.4rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.stand :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
	}
	.stand .status {
		margin-left: 0.25rem;
	}
	.auszug {
		align-self: start;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 4;
		line-clamp: 4;
		overflow: hidden;
		margin-top: 0.25rem;
		padding-top: 0.75rem;
		border-top: 1px solid hsl(var(--color-border-primary));
		font-size: 1rem;
		line-height: 1.65;
		color: hsl(var(--color-text-secondary));
		text-wrap: pretty;
	}
	.wer {
		margin-right: 0.4rem;
		color: var(--c);
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.fuss {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 1rem;
		padding-top: 0.5rem;
	}
	.stimmen {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.15rem 0.75rem;
		font-size: 0.9375rem;
		font-weight: 600;
	}
	.stimmen li {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
	}
	.punkt {
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--c);
	}
	.mehr {
		color: hsl(var(--color-text-tertiary));
	}
	.arbeit {
		display: flex;
		gap: 0.75rem;
		font-size: 0.9375rem;
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
	}
	/* oben rechts, über dem Link des Blatts */
	.neu-tx {
		position: absolute;
		z-index: 1;
		top: 0.625rem;
		right: 0.625rem;
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

	/* Löschen: leise, bis man darauf zeigt; im Blatt unten rechts, in der Zeile am Ende */
	.weg {
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
	.karte .weg {
		position: relative;
		z-index: 1;
		margin: -0.5rem -0.75rem -0.5rem auto;
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
		cursor: pointer;
	}
	.willkommen:hover {
		border-color: hsl(var(--color-neutral-100));
	}
	.willkommen:focus-within {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.willkommen :global(svg) {
		width: 2.75rem;
		height: 2.75rem;
		color: hsl(var(--color-text-secondary));
	}
	.willkommen strong {
		font-size: 1.25rem;
	}
	.willkommen > span:not(.knopf) {
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

	@media (prefers-reduced-motion: reduce) {
		.karte {
			transition: none;
		}
		.karte:hover {
			transform: none;
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
	}
</style>
