<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Eine Live-Aufnahme groß: oben woher sie kommt und was sie ist, mit Kopieren, Teilen,
     Nachtranskribieren und Löschen; darunter der Text als Manuskript, Satz für Satz mit der Zeit
     und, bei gemischter Aufnahme, der Quelle in der Randspalte. Läuft sie, scrollt der Text mit.
     Ist ihre Datei gespeichert, lässt sie sich im Kopf abspielen: der Satz, der gerade läuft, ist
     markiert, und ein Klick auf die Zeit eines Satzes spielt ab dort. -->
<script>
	import { tick } from 'svelte';
	import { dauer } from '$lib/api.js';
	import Teilen from '$lib/Teilen.svelte';
	import Abspielen from './Abspielen.svelte';
	import Platzhalter from './Platzhalter.svelte';
	import { art, laenge, LINIE, live, QUELLEN, sprachname, tag, text, uhrzeit, vonHand, WARTET, wer } from './stand.svelte.js';
	import { stellenIn, stuecke } from './suche.js';

	/** @typedef {import('./stand.svelte.js').Notiz} Notiz */

	/** @type {{ n: Notiz, q: string, onzurueck?: () => void }} */
	let { n, q, onzurueck } = $props();

	const hand = $derived(vonHand(n));
	const laeuft = $derived(!n.ended);
	const suche = $derived(q.trim());
	/** Sprache, die schon gehört, aber noch nicht erkannt ist */
	const gehoert = $derived(laeuft ? live.gehoert[n.id] : undefined);

	let meldung = $state('');
	let kopiert = $state(false);
	let eingereiht = $state('');
	// eine andere Aufnahme: die Meldungen gehören zur vorigen
	$effect.pre(() => {
		n.id;
		meldung = eingereiht = '';
		kopiert = false;
	});

	async function kopieren() {
		try {
			await navigator.clipboard.writeText(text(n));
			kopiert = true;
			setTimeout(() => (kopiert = false), 2000);
		} catch {
			meldung = 'Kopieren ging nicht.';
		}
	}

	/** @type {HTMLElement | undefined} */
	let blatt = $state();

	/** Der Text scrollt mit, solange man unten ist; wer zurückscrollt, liest in Ruhe.
	 * @param {HTMLElement} el */
	function mitscrollen(el) {
		let unten = true;
		const merken = () => (unten = el.scrollHeight - el.scrollTop - el.clientHeight < 48);
		el.addEventListener('scroll', merken);
		$effect(() => {
			n.lines.length;
			n.partial;
			gehoert;
			if (unten && !suche) tick().then(() => (el.scrollTop = el.scrollHeight));
		});
		// eine andere Aufnahme beginnt oben, eine laufende unten
		$effect(() => {
			n.id;
			tick().then(() => {
				el.scrollTop = laeuft ? el.scrollHeight : 0;
				unten = laeuft;
			});
		});
		return () => el.removeEventListener('scroll', merken);
	}

	// beim Suchen steht der erste Treffer im Blick
	$effect(() => {
		n.id;
		if (!suche) return;
		tick().then(() => blatt?.querySelector('mark')?.scrollIntoView({ block: 'center', behavior: 'instant' }));
	});

	/** die Zeilen mit Quelle beim Wechsel und Trefferstellen */
	const zeilen = $derived(
		n.lines.map((l, i) => ({
			...l,
			wechsel: !!l.source && l.source !== n.lines[i - 1]?.source,
			stellen: suche ? stellenIn(suche, l.text) : null
		}))
	);
	const treffer = $derived(zeilen.filter((z) => z.stellen).length);

	/** die gespeicherte Datei lässt sich abspielen, sobald die Aufnahme zu Ende ist */
	const abspielbar = $derived(!!n.ended && !!n.audio);
	/** @type {ReturnType<typeof Abspielen> | undefined} */
	let spieler = $state();
	let zeit = $state(0);
	let gespielt = $state(false);
	/** der Satz an der Stelle, die abgespielt wird; -1 vor dem ersten Abspielen */
	const spielt = $derived(abspielbar && gespielt ? n.lines.findLastIndex((l) => l.start <= zeit + 0.05) : -1);

	// der Satz, der gerade läuft, bleibt im Blick
	$effect(() => {
		if (spielt < 0) return;
		tick().then(() => blatt?.querySelector('.satz.spielt')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }));
	});
</script>

<article class="aufnahme" aria-labelledby="titel-{n.id}">
	<header class="kopf">
		{#if onzurueck}
			<button class="zurueck" type="button" onclick={onzurueck}>
				<IconMaterialSymbolsArrowBackRounded aria-hidden="true" />Alle Aufnahmen
			</button>
		{/if}
		<div class="titel">
			<h1 id="titel-{n.id}">{tag(n.started, live.jetzt)}, {uhrzeit(n.started)}</h1>
			{#if laeuft}
				<span class="status live"><span class="punkt" aria-hidden="true"></span>läuft</span>
			{:else if n.error}
				<span class="status fehler">abgebrochen</span>
			{/if}
		</div>
		<p class="meta">
			<span class="herkunft" class:hand>
				{#if n.kind === 'diktat'}
					<IconMaterialSymbolsKeyboardOutlineRounded aria-hidden="true" />Diktiert per Tastenkürzel
				{:else if hand}
					<IconMaterialSymbolsBackHandOutlineRounded aria-hidden="true" />Von Hand, auf dieser Seite
				{:else}
					<IconMaterialSymbolsCableRounded aria-hidden="true" /><span
						>Per API: <b>{wer(n)}</b> {art(n)}</span
					>
				{/if}
			</span>
			<span>Länge <b>{n.loading || n.waiting ? '–' : dauer(laenge(n, live.jetzt))}</b></span>
			{#if n.source}<span>Quelle <b>{QUELLEN[n.source]}</b></span>{/if}
			<span>Sprache <b>{sprachname(n.language)}</b></span>
			{#if n.transient}<span class="fluechtig">Wird nicht gespeichert</span>{/if}
			{#if suche && n.lines.length}
				<span class="treffer">{treffer === 1 ? 'ein Satz' : treffer ? `${treffer} Sätze` : 'kein Satz'} mit „{suche}“</span>
			{/if}
		</p>
		<div class="tun">
			{#if n.ended && n.audio}
				<button class="pille dunkel" type="button" onclick={async () => (eingereiht = await live.nachtranskribieren(n))}>
					<IconMaterialSymbolsPlaylistAddRounded aria-hidden="true" />Vollständig transkribieren
				</button>
			{/if}
			{#if n.lines.length}
				<button class="pille" type="button" onclick={kopieren}>
					{#if kopiert}<IconMaterialSymbolsCheckRounded aria-hidden="true" />Kopiert{:else}<IconMaterialSymbolsContentCopyOutlineRounded
							aria-hidden="true"
						/>Text kopieren{/if}
				</button>
				<span class="teilen"
					><Teilen text={text(n)} beschreibung="Aufnahme {uhrzeit(n.started)} teilen" onmeldung={(m) => (meldung = m)} /></span
				>
			{/if}
			{#if n.ended}
				<button class="pille leise" type="button" title="Aufnahme {tag(n.started, live.jetzt)}, {uhrzeit(n.started)} löschen" onclick={() => live.loeschen(n)}>
					<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />Löschen
				</button>
			{/if}
		</div>
		{#if abspielbar && n.audio}
			<div class="abspielen"><Abspielen bind:this={spieler} datei={n.audio} bind:zeit bind:gespielt /></div>
		{/if}
		{#if meldung || eingereiht}<p class="meldung" role="status">{eingereiht || meldung}</p>{/if}
	</header>

	<!-- mit Tab erreichbar, damit es sich mit den Pfeiltasten scrollen lässt -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="tisch" tabindex="0" role="log" aria-live={laeuft ? 'polite' : 'off'} aria-label="Text der Aufnahme" {@attach mitscrollen}>
		<div class="blatt" bind:this={blatt}>
			{#if n.error}
				<p class="fehler" role="alert"><IconMaterialSymbolsErrorOutlineRounded aria-hidden="true" />{n.error}</p>
			{/if}
			{#each zeilen as z, i (i)}
				<div class="satz" class:wechsel={z.wechsel} class:treffer={z.stellen} class:spielt={i === spielt}>
					<div class="rand">
						{#if z.wechsel && z.source}<span class="quelle {z.source}">{LINIE[z.source]}</span>{/if}
						{#if abspielbar}
							<button class="zeit" type="button" title="Ab hier abspielen" onclick={() => spieler?.ab(z.start)}
								><IconMaterialSymbolsPlayArrowRounded aria-hidden="true" />{dauer(z.start)}</button
							>
						{:else}
							<span class="zeit">{dauer(z.start)}</span>
						{/if}
					</div>
					<p>
						{#if z.stellen}{#each stuecke(z.text, z.stellen) as s, k (k)}{#if s.an}<mark>{s.t}</mark>{:else}{s.t}{/if}{/each}{:else}{z.text}{/if}
					</p>
				</div>
			{/each}
			{#if n.partial || gehoert}
				{@const wechsel = !!n.partial && !!n.partialSource && n.partialSource !== n.lines.at(-1)?.source}
				<div class="satz vorlaeufig" class:wechsel aria-hidden="true">
					<div class="rand">
						{#if wechsel && n.partialSource}<span class="quelle {n.partialSource}">{LINIE[n.partialSource]}</span>{/if}
					</div>
					<p>{n.partial}{#if n.partial && gehoert}{' '}{/if}{#if gehoert}<Platzhalter g={gehoert} />{/if}<span class="cursor"></span></p>
				</div>
			{:else if laeuft && !n.lines.length}
				<p class="warten">
					{n.loading ? 'Lädt die Erkennung …' : n.waiting ? WARTET[n.source ?? 'mic'] : 'Hört zu … sprich einfach los.'}
				</p>
			{:else if !n.lines.length && !n.error}
				<p class="warten">Es wurde nichts erkannt.</p>
			{/if}
			{#if n.audio}
				<p class="datei">
					<IconMaterialSymbolsSaveOutlineRounded aria-hidden="true" />
					<span
						>Aufnahme gespeichert als „{n.audio}“.{n.ended && !eingereiht
							? ' Vollständig transkribiert bekommt sie Sprecher und lässt sich im Editor korrigieren.'
							: ''}</span
					>
				</p>
			{/if}
		</div>
	</div>
</article>

<style>
	.aufnahme {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		min-height: 0;
		min-width: 0;
	}

	/* Kopf wie im Editor: Papierband mit Haarlinie, Titel und Stand, rechts die Pillen */
	.kopf {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.25rem 1.5rem;
		padding: 0.875rem 1.5rem 0.875rem 1.75rem;
		background: hsl(var(--color-bg-primary));
		border-bottom: 1px solid hsl(var(--color-border-primary));
	}
	.titel {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.25rem 0.75rem;
		min-width: 0;
	}
	h1 {
		font-size: 1.375rem;
		letter-spacing: -0.01em;
	}
	.meta {
		grid-column: 1;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.125rem 1.25rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.meta b {
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		font-variant-numeric: tabular-nums;
	}
	.herkunft {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		font-weight: 600;
		color: hsl(var(--color-text-primary));
	}
	.herkunft :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
		color: hsl(var(--color-text-secondary));
	}
	.treffer,
	.fluechtig {
		color: hsl(var(--color-text-primary));
	}
	.tun {
		grid-column: 2;
		grid-row: 1 / span 2;
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		align-items: center;
		gap: 0.5rem;
	}
	.meldung {
		grid-column: 1 / -1;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}

	/* Pillen wie „Bedienung“ im Editor-Kopf: 2 px Tinte, Hover invertiert */
	.pille {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1.125rem 0 0.875rem;
		border: 2px solid hsl(var(--color-neutral-100));
		border-radius: 2rem;
		background: hsl(var(--color-bg-primary));
		font-weight: 700;
		white-space: nowrap;
		cursor: pointer;
		transition:
			background-color 0.15s,
			color 0.15s;
	}
	.pille:hover,
	.pille.dunkel {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	.pille.dunkel:hover {
		background: color-mix(in oklab, hsl(var(--color-neutral-100)) 80%, hsl(var(--color-neutral-0)));
	}
	.pille.leise {
		border-color: transparent;
		background: transparent;
	}
	.pille.leise:hover {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	.pille :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}
	.teilen {
		display: contents;
	}

	.zurueck {
		grid-column: 1 / -1;
		justify-self: start;
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		margin: -0.5rem 0 0 -0.5rem;
		padding: 0 0.75rem 0 0.5rem;
		border-radius: 2rem;
		font-weight: 600;
		cursor: pointer;
	}
	.zurueck:hover {
		background: hsl(var(--color-bg-secondary));
	}

	.status {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.1rem 0.7rem;
		border-radius: 2rem;
		font-size: 0.875rem;
		font-weight: 700;
	}
	.status.live {
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
	}
	.status.fehler {
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
	}
	.punkt {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: currentColor;
		animation: atmen 1.2s ease-in-out infinite;
	}

	/* Tisch und Blatt wie im Editor: Papier auf Nebelgrau, das Blatt scrollt */
	.tisch {
		overflow-y: auto;
		padding: 1.25rem 1.5rem 2rem;
		scrollbar-color: hsl(var(--color-neutral-50) / 0.6) transparent;
	}
	.tisch:focus-visible {
		outline-offset: -3px;
	}
	.blatt {
		max-width: 1040px;
		margin: 0 auto;
		padding: 1.5rem 2.75rem 2.5rem;
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.05),
			0 12px 32px -20px hsl(var(--color-neutral-100) / 0.2);
	}

	/* Satz für Satz: Randspalte mit Zeit (und Quelle beim Wechsel), daneben der Text */
	.satz {
		display: grid;
		grid-template-columns: 7.5rem minmax(0, 42rem);
		column-gap: 1.75rem;
		padding: 0.1875rem 0;
	}
	/* Platz für die Markierung beim Abspielen, die beim Weitergehen ausblasst */
	.satz {
		margin-inline: -0.75rem;
		padding-inline: 0.75rem;
		border-radius: 0.375rem;
		transition:
			background-color 1.2s,
			box-shadow 1.2s;
	}
	.satz.wechsel {
		margin-top: 0.75rem;
		padding-top: 0.875rem;
		border-top: 1px solid hsl(var(--color-border-primary));
	}
	.satz.wechsel:first-child {
		margin-top: 0;
		padding-top: 0.1875rem;
		border-top: 0;
	}
	.rand {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		padding-top: 0.375rem;
		text-align: right;
	}
	.zeit {
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-tertiary));
	}
	/* abspielbar: die Zeit ist ein Knopf, der Pfeil davor erscheint beim Zeigen */
	button.zeit {
		display: inline-flex;
		align-items: center;
		gap: 0.125rem;
		margin: -0.125rem -0.375rem 0 0;
		padding: 0.125rem 0.375rem;
		border-radius: 0.375rem;
		cursor: pointer;
	}
	button.zeit :global(svg) {
		width: 1rem;
		height: 1rem;
		opacity: 0;
		transition: opacity 0.15s;
	}
	button.zeit:hover,
	button.zeit:focus-visible {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	button.zeit:hover :global(svg),
	button.zeit:focus-visible :global(svg),
	.satz.spielt button.zeit :global(svg) {
		opacity: 1;
	}
	/* der Satz, der gerade abgespielt wird: Tintenstrich am Rand */
	.satz.spielt {
		background: hsl(var(--color-bg-secondary));
		box-shadow: inset 3px 0 0 hsl(var(--color-neutral-100));
		transition: none;
	}
	.satz.spielt button.zeit {
		color: hsl(var(--color-text-primary));
	}
	.abspielen {
		grid-column: 1 / -1;
		margin-top: 0.5rem;
	}
	/* Quellen einer gemischten Aufnahme in den ersten Sprecherfarben des Editors */
	.quelle {
		font-size: 0.875rem;
		font-weight: 700;
		line-height: 1.3;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--spk0);
	}
	.quelle.system {
		color: var(--spk1);
	}
	.satz p {
		font-size: 1.25rem;
		line-height: 1.75;
		text-wrap: pretty;
		white-space: pre-line; /* angesagte Zeilenumbrüche und Absätze */
	}
	.vorlaeufig p {
		color: hsl(var(--color-text-secondary));
	}
	/* was gerade gesprochen wird: ein atmender Signalpunkt am Ende */
	.cursor {
		display: inline-block;
		width: 0.55em;
		height: 0.55em;
		margin-left: 0.35em;
		border-radius: 50%;
		background: hsl(var(--color-green-100));
		animation: atmen 1.2s ease-in-out infinite;
	}
	mark {
		background: hsl(var(--color-green-50));
		color: inherit;
		border-radius: 3px;
	}
	.warten {
		padding: 1rem 0;
		font-size: 1.25rem;
		color: hsl(var(--color-text-tertiary));
		font-style: italic;
		text-align: center;
	}
	.fehler {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		margin-bottom: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
		font-weight: 600;
	}
	.fehler :global(svg) {
		flex: none;
		width: 1.375rem;
		height: 1.375rem;
	}
	.datei {
		display: flex;
		gap: 0.5rem;
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid hsl(var(--color-border-primary));
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.datei :global(svg) {
		flex: none;
		width: 1.25rem;
		height: 1.25rem;
		margin-top: 0.125rem;
	}

	@keyframes atmen {
		50% {
			opacity: 0.25;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.punkt,
		.cursor {
			animation: none;
		}
	}

	@media (max-width: 1100px) {
		.kopf {
			grid-template-columns: minmax(0, 1fr);
		}
		.tun {
			grid-column: 1;
			grid-row: auto;
			justify-content: flex-start;
			margin-top: 0.5rem;
		}
	}
	@media (max-width: 720px) {
		.kopf {
			padding: 0.75rem 1rem;
		}
		.tisch {
			padding: 0.75rem 0.5rem 1.5rem;
		}
		.blatt {
			padding: 1rem 1.125rem 1.5rem;
		}
		.satz {
			grid-template-columns: minmax(0, 1fr);
		}
		.rand {
			flex-direction: row;
			align-items: baseline;
			gap: 0.75rem;
			padding-top: 0;
		}
		.satz p {
			font-size: 1.1875rem;
		}
		/* schmal steht die Zeit nur beim Wechsel der Quelle, sonst nähme sie jedem Satz eine Zeile */
		.satz:not(.wechsel) .rand {
			display: none;
		}
	}
</style>
