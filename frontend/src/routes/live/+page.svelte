<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Live: Sprache sofort als Text. Links alle Aufnahmen (von Hand auf dieser Seite gestartet oder per
     API von einem Programm), durchsuchbar, gefiltert und sortiert; rechts die gewählte groß; unten die
     Aufnahmeleiste mit Aufnehmen, Pegeln und „Einrichten“. „Programme verbinden“ hat eine eigene
     Ansicht. Den Stand schickt der Server über /api/live/events (internal/live). -->
<script>
	import { onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { afterNavigate, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import Aufnahme from '$lib/live/Aufnahme.svelte';
	import Aufnahmeleiste from '$lib/live/Aufnahmeleiste.svelte';
	import Liste from '$lib/live/Liste.svelte';
	import Verbinden from '$lib/live/Verbinden.svelte';
	import { live } from '$lib/live/stand.svelte.js';

	/** @type {string | null} */
	let gewaehlt = $state(page.url.searchParams.get('aufnahme'));
	let verbinden = $state(page.url.searchParams.has('verbinden'));
	let q = $state('');

	/** schmal: nur eine Spalte, entweder die Liste oder eine Aufnahme */
	const schmal = new MediaQuery('max-width: 900px');
	let liste = $state(!page.url.searchParams.has('aufnahme') && !page.url.searchParams.has('verbinden'));

	const notiz = $derived(live.notizen?.find((n) => n.id === gewaehlt) ?? null);

	/** @param {string} id @param {boolean} [bleiben] in der schmalen Ansicht bleibt die Liste stehen */
	function waehlen(id, bleiben = false) {
		gewaehlt = id;
		verbinden = false;
		if (!bleiben) liste = false;
	}

	// Keine oder eine verschwundene Aufnahme gewählt: die laufende, sonst die neueste
	$effect(() => {
		if (!live.notizen || verbinden || notiz) return;
		const neueste = live.notizen.toSorted((a, b) => b.started.localeCompare(a.started))[0];
		gewaehlt = (live.laufend ?? neueste)?.id ?? null;
	});

	// Eine eigene Aufnahme beginnt: sie steht groß da
	let zuletztGestartet = '';
	$effect(() => {
		const id = live.mic.dauer;
		if (id && id !== zuletztGestartet) {
			zuletztGestartet = id;
			waehlen(id);
		}
	});

	// replaceState geht erst, wenn der Router fertig ist; davor bricht es das Einhängen der Seite ab.
	// afterNavigate kommt beim Laden kurz davor, der Effekt läuft danach.
	let bereit = $state(false);
	afterNavigate(() => (bereit = true));

	// Die Wahl steht in der Adresse, damit Neuladen dieselbe Aufnahme zeigt. Andere Teile der Adresse
	// (etwa „?neu“) bleiben, wie sie sind.
	$effect(() => {
		const andere = location.search
			.slice(1)
			.split('&')
			.filter((teil) => teil && !/^(aufnahme|verbinden)(=|$)/.test(teil));
		const eigene = verbinden ? ['verbinden'] : gewaehlt ? [`aufnahme=${encodeURIComponent(gewaehlt)}`] : [];
		const teile = [...andere, ...eigene];
		const neu = teile.length ? `?${teile.join('&')}` : '';
		if (bereit && neu !== location.search) replaceState(location.pathname + neu, {});
	});

	onMount(() => live.verbinden());
</script>

<svelte:head><title>Live · Transcribe by Moliri</title></svelte:head>

<main class="live" class:schmal={schmal.current} class:zeigt-liste={schmal.current && liste}>
	<Liste
		{gewaehlt}
		{verbinden}
		bind:q
		onwaehlen={waehlen}
		onverbinden={() => ((verbinden = true), (liste = false))}
	/>

	<div class="haupt">
		{#if !live.verbunden}
			<p class="getrennt" role="alert">
				<IconMaterialSymbolsErrorOutlineRounded aria-hidden="true" />Keine Verbindung zum Programm. Läuft es noch?
			</p>
		{/if}

		{#if verbinden}
			<Verbinden onzurueck={schmal.current ? () => (liste = true) : undefined} />
		{:else if notiz}
			<Aufnahme n={notiz} {q} onzurueck={schmal.current ? () => (liste = true) : undefined} />
		{:else if live.notizen}
			<div class="willkommen">
				<h1>Sprache sofort als Text</h1>
				<p>
					Starte unten eine Aufnahme: übers Mikrofon und auf Wunsch mit dem Ton, den der Rechner abspielt, etwa die
					anderen in einem Online-Meeting. Der Text erscheint hier, während gesprochen wird.
				</p>
				<p>
					Oder lass Programme wie Moliri Sprache schicken, über die
					<button class="link" type="button" onclick={() => (verbinden = true)}>Live-API</button>. Jede Anfrage
					{live.apiSpeichern ? 'bleibt links als Aufnahme stehen.' : 'steht links, solange sie läuft.'}
				</p>
			</div>
		{/if}
	</div>

	<div class="unten"><Aufnahmeleiste /></div>
</main>

<style>
	/* füllt die Höhe unter der App-Leiste wie der Editor: Seitenleiste, Hauptfläche, Aufnahmeleiste */
	.live {
		height: calc(100dvh - 60px);
		display: grid;
		grid-template-columns: 22rem minmax(0, 1fr);
		grid-template-rows: minmax(0, 1fr) auto;
		background: hsl(var(--page-bg));
	}
	.haupt {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		min-height: 0;
		min-width: 0;
	}
	.haupt > :global(:last-child) {
		grid-row: 2;
	}
	.unten {
		grid-column: 1 / -1;
	}

	/* schmal: eine Spalte, Liste oder Aufnahme */
	.live.schmal {
		grid-template-columns: minmax(0, 1fr);
	}
	.live.schmal > :global(.leiste) {
		display: none;
		border-right: 0;
	}
	.live.schmal.zeigt-liste > :global(.leiste) {
		display: grid;
	}
	.live.schmal.zeigt-liste .haupt {
		display: none;
	}

	.getrennt {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.625rem 1.5rem;
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
		font-weight: 600;
	}
	.getrennt :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}

	.willkommen {
		align-self: center;
		justify-self: center;
		max-width: 38rem;
		padding: 2rem 1.5rem;
		display: grid;
		gap: 0.875rem;
	}
	.willkommen h1 {
		font-size: 2rem;
		letter-spacing: -0.01em;
		margin-bottom: 0.25rem;
	}
	.willkommen p {
		font-size: 1.125rem;
		color: hsl(var(--color-text-secondary));
		text-wrap: pretty;
	}
	.link {
		display: inline;
		font: inherit;
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		text-decoration-color: hsl(var(--color-green-100));
		text-decoration-thickness: 2px;
		cursor: pointer;
	}
</style>
