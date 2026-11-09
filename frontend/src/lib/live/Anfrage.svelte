<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Eine Anfrage eines Programms an einen Dienst groß (Texterkennung, Filter in eigenen Worten, Suche
     nach Bedeutung): oben wer fragte, welche KI antwortete und wie lange es dauerte, mit Kopieren und
     Löschen; darunter die Anfrage, die Antwort und was beim Nachsehen hilft (Zeiten, Größen, Tokens). -->
<script>
	import { anfrage, art, DIENSTE, laenge, live, sekunden, tag, uhrzeit, wer } from './stand.svelte.js';

	/** @typedef {import('./stand.svelte.js').Notiz} Notiz */

	/** @type {{ n: Notiz, onzurueck?: () => void }} */
	let { n, onzurueck } = $props();

	const laeuft = $derived(!n.ended);
	const dienst = $derived(anfrage(n) ? DIENSTE[/** @type {import('./stand.svelte.js').Dienst} */ (n.kind)] : '');

	/** Die Antwort des Sprachmodells ist JSON: eingerückt liest sie sich besser. */
	const antwort = $derived.by(() => {
		const r = n.response ?? '';
		if (!r.trimStart().startsWith('{')) return r;
		try {
			return JSON.stringify(JSON.parse(r), null, 2);
		} catch {
			return r;
		}
	});

	let meldung = $state('');
	/** @type {'' | 'anfrage' | 'antwort'} */
	let kopiert = $state('');
	$effect.pre(() => {
		n.id;
		meldung = kopiert = '';
	});

	/** @param {'anfrage' | 'antwort'} was */
	async function kopieren(was) {
		try {
			await navigator.clipboard.writeText(was === 'anfrage' ? (n.request ?? '') : antwort);
			kopiert = was;
			setTimeout(() => (kopiert = ''), 2000);
		} catch {
			meldung = 'Kopieren ging nicht.';
		}
	}

	/** die Uhrzeit auf die Sekunde, zum Abgleich mit dem Protokoll des Programms
	 * @param {string} iso */
	const genau = (iso) => new Date(iso).toLocaleTimeString('de-DE');
</script>

<article class="anfrage" aria-labelledby="titel-{n.id}">
	<header class="kopf">
		{#if onzurueck}
			<button class="zurueck" type="button" onclick={onzurueck}>
				<IconMaterialSymbolsArrowBackRounded aria-hidden="true" />Alle Aufnahmen
			</button>
		{/if}
		<div class="titel">
			<h1 id="titel-{n.id}">{dienst}, {tag(n.started, live.jetzt)}, {uhrzeit(n.started)}</h1>
			{#if laeuft}
				<span class="status live"><span class="punkt" aria-hidden="true"></span>läuft</span>
			{:else if n.error}
				<span class="status fehler">Fehler</span>
			{/if}
		</div>
		<p class="meta">
			<span class="herkunft">
				{#if n.kind === 'ocr'}<IconMaterialSymbolsDocumentScannerOutlineRounded aria-hidden="true" />{:else if n.kind === 'filter'}<IconMaterialSymbolsAutoAwesomeOutlineRounded
						aria-hidden="true"
					/>{:else}<IconMaterialSymbolsManageSearchRounded aria-hidden="true" />{/if}<span
					>Per API: <b>{wer(n)}</b> {art(n)}</span
				>
			</span>
			{#if n.model}<span>KI <b>{n.model}</b></span>{/if}
			<span>Dauer <b>{sekunden(laenge(n, live.jetzt))}</b></span>
			{#if n.transient}<span class="fluechtig">Wird nicht gespeichert</span>{/if}
		</p>
		<div class="tun">
			{#if n.ended && n.response}
				<button class="pille" type="button" onclick={() => kopieren('antwort')}>
					{#if kopiert === 'antwort'}<IconMaterialSymbolsCheckRounded aria-hidden="true" />Kopiert{:else}<IconMaterialSymbolsContentCopyOutlineRounded
							aria-hidden="true"
						/>Antwort kopieren{/if}
				</button>
			{/if}
			{#if n.ended}
				<button class="pille leise" type="button" title="Anfrage {tag(n.started, live.jetzt)}, {uhrzeit(n.started)} löschen" onclick={() => live.loeschen(n)}>
					<IconMaterialSymbolsDeleteOutlineRounded aria-hidden="true" />Löschen
				</button>
			{/if}
		</div>
		{#if meldung}<p class="meldung" role="status">{meldung}</p>{/if}
	</header>

	<!-- mit Tab erreichbar, damit es sich mit den Pfeiltasten scrollen lässt -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="tisch" tabindex="0" role="region" aria-label="Anfrage und Antwort">
		<div class="blatt">
			{#if n.error}
				<p class="fehler" role="alert"><IconMaterialSymbolsErrorOutlineRounded aria-hidden="true" />{n.error}</p>
			{/if}

			<section aria-labelledby="h-anfrage-{n.id}">
				<div class="abschnitt">
					<h2 id="h-anfrage-{n.id}">Anfrage</h2>
					{#if n.request}
						<button class="klein" type="button" onclick={() => kopieren('anfrage')}>
							{kopiert === 'anfrage' ? 'Kopiert' : 'Kopieren'}
						</button>
					{/if}
				</div>
				<pre>{n.request || '–'}</pre>
			</section>

			<section aria-labelledby="h-antwort-{n.id}">
				<div class="abschnitt"><h2 id="h-antwort-{n.id}">Antwort</h2></div>
				{#if laeuft}
					<p class="warten"><span class="punkt" aria-hidden="true"></span>{n.partial || 'Läuft …'}</p>
				{:else if antwort}
					<pre>{antwort}</pre>
				{:else}
					<p class="warten">{n.error ? 'Keine Antwort.' : 'Leere Antwort.'}</p>
				{/if}
			</section>

			<section aria-labelledby="h-details-{n.id}">
				<div class="abschnitt"><h2 id="h-details-{n.id}">Details</h2></div>
				<dl>
					<dt>Dienst</dt>
					<dd>{dienst}</dd>
					{#if n.model}<dt>KI</dt><dd>{n.model}</dd>{/if}
					<dt>Programm</dt>
					<dd>{n.client || 'ohne Namen'}</dd>
					<dt>Gestartet</dt>
					<dd>{genau(n.started)}</dd>
					{#if n.ended}<dt>Fertig</dt><dd>{genau(n.ended)}</dd>{/if}
					<dt>Dauer</dt>
					<dd>{sekunden(laenge(n, live.jetzt))}</dd>
					{#each n.facts ?? [] as f, i (i)}
						<dt>{f.label}</dt>
						<dd>{f.value}</dd>
					{/each}
				</dl>
			</section>
		</div>
	</div>
</article>

<style>
	.anfrage {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		min-height: 0;
		min-width: 0;
	}

	/* Kopf wie bei einer Aufnahme */
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
	.pille:hover {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
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

	/* Tisch und Blatt wie bei einer Aufnahme */
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
		display: grid;
		gap: 1.75rem;
		background: hsl(var(--color-bg-primary));
		border-radius: 0.75rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.05),
			0 12px 32px -20px hsl(var(--color-neutral-100) / 0.2);
	}
	section {
		display: grid;
		gap: 0.625rem;
		min-width: 0;
	}
	.abschnitt {
		display: flex;
		align-items: baseline;
		gap: 1rem;
	}
	h2 {
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: hsl(var(--color-text-secondary));
	}
	.klein {
		font-size: 0.875rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		cursor: pointer;
	}
	.klein:hover {
		color: hsl(var(--color-text-primary));
	}
	pre {
		max-height: 28rem;
		overflow: auto;
		margin: 0;
		padding: 0.875rem 1rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
		font-family: var(--font-mono);
		font-size: 0.875rem;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.warten {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		color: hsl(var(--color-text-tertiary));
		font-style: italic;
	}
	.warten .punkt {
		color: hsl(var(--color-green-100));
	}
	dl {
		display: grid;
		grid-template-columns: max-content minmax(0, 1fr);
		gap: 0.375rem 1.5rem;
		font-size: 0.9375rem;
	}
	dt {
		color: hsl(var(--color-text-secondary));
	}
	dd {
		margin: 0;
		font-variant-numeric: tabular-nums;
		overflow-wrap: anywhere;
	}
	.fehler {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
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

	@keyframes atmen {
		50% {
			opacity: 0.35;
		}
	}
</style>
