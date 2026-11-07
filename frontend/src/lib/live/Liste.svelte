<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Seitenleiste der Live-Seite: alle Aufnahmen, durchsuchbar (unscharf, im ganzen Text), nach
     Herkunft gefiltert (von Hand oder per API) und sortiert; unten „Programme verbinden“. -->
<script>
	import { dauer } from '$lib/api.js';
	import { live, laenge, QUELLEN, tag, uhrzeit, vonHand, WARTET, wer } from './stand.svelte.js';
	import { ausschnitt, HERKUNFT, passt, SORTIERUNG, sortieren, stellenIn, stuecke, suchen } from './suche.js';

	/** @typedef {import('./stand.svelte.js').Notiz} Notiz */

	/** bleiben: nur zeigen, in der schmalen Ansicht bleibt die Liste stehen
	 * @type {{ gewaehlt: string | null, verbinden: boolean, q: string, onwaehlen: (id: string, bleiben?: boolean) => void,
	 *   onverbinden: () => void }} */
	let { gewaehlt, verbinden, q = $bindable(), onwaehlen, onverbinden } = $props();

	/** @type {import('./suche.js').Herkunft} */
	let herkunft = $state('alle');
	/** @type {import('./suche.js').Sortierung} */
	let sortierung = $state('neu');

	const alle = $derived(live.notizen ?? []);
	const zahl = $derived({
		alle: alle.length,
		hand: alle.filter(vonHand).length,
		api: alle.filter((n) => !vonHand(n)).length
	});
	const treffer = $derived(suchen(alle, q));
	const sucht = $derived(!!q.trim());

	/** gefiltert, gesucht und sortiert; beim Suchen die besten Treffer zuerst */
	const sichtbar = $derived.by(() => {
		const l = alle.filter((n) => passt(n, herkunft) && (!sucht || treffer.has(n.id)));
		const s = sortieren(l, sortierung, live.jetzt);
		return sucht ? s.toSorted((a, b) => (treffer.get(b.id)?.score ?? 0) - (treffer.get(a.id)?.score ?? 0)) : s;
	});

	// Beim Suchen steht der beste Treffer groß da, wenn die gezeigte Aufnahme nicht mehr dazugehört
	$effect(() => {
		if (sucht && !verbinden && sichtbar.length && !sichtbar.some((n) => n.id === gewaehlt)) onwaehlen(sichtbar[0].id, true);
	});

	/** nach Tagen gruppiert, solange nach Zeit sortiert wird und nicht gesucht */
	const gruppen = $derived.by(() => {
		if (sucht || sortierung === 'lang') return [{ titel: '', notizen: sichtbar }];
		/** @type {{ titel: string, notizen: Notiz[] }[]} */
		const g = [];
		for (const n of sichtbar) {
			const t = tag(n.started, live.jetzt);
			if (g.at(-1)?.titel === t) g.at(-1)?.notizen.push(n);
			else g.push({ titel: t, notizen: [n] });
		}
		return g;
	});

	/** die Vorschau: beim Suchen der getroffene Satz, sonst der Anfang
	 * @param {Notiz} n */
	function vorschau(n) {
		const t = treffer.get(n.id);
		if (t && t.zeile >= 0) {
			const satz = n.lines[t.zeile].text;
			const a = ausschnitt(satz, stellenIn(q, satz) ?? []);
			return stuecke(a.satz, a.stellen);
		}
		const anfang = n.lines.map((l) => l.text).join(' ') || n.partial || '';
		return anfang ? [{ t: anfang, an: false }] : [];
	}

	/** @type {HTMLInputElement | undefined} */
	let feld = $state();

	/** Pfeile wandern durch die Aufnahmen und zeigen sie gleich.
	 * @param {KeyboardEvent} e */
	function taste(e) {
		if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
		const i = sichtbar.findIndex((n) => n.id === gewaehlt);
		const n = sichtbar[Math.max(0, Math.min(sichtbar.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1)))];
		if (!n) return;
		e.preventDefault();
		onwaehlen(n.id, true);
		/** @type {HTMLElement | null} */ (
			/** @type {HTMLElement} */ (e.currentTarget).querySelector(`[data-id="${n.id}"]`)
		)?.focus();
	}

	/** Strg+F und „/“ springen ins Suchfeld
	 * @param {KeyboardEvent} e */
	function global(e) {
		const t = /** @type {HTMLElement} */ (e.target);
		const tippt = t.closest('input, select, textarea, [contenteditable]');
		if ((e.key === 'f' && (e.ctrlKey || e.metaKey)) || (e.key === '/' && !tippt)) {
			e.preventDefault();
			feld?.focus();
			feld?.select();
		}
	}
</script>

<svelte:window onkeydown={global} />

<aside class="leiste" aria-label="Aufnahmen">
	<div class="finden">
		<label class="suche">
			<span class="sr">In allen Aufnahmen suchen</span>
			<IconMaterialSymbolsSearchRounded aria-hidden="true" />
			<input
				bind:this={feld}
				bind:value={q}
				class="feld"
				type="search"
				placeholder="In allen Aufnahmen suchen"
				autocomplete="off"
				spellcheck="false"
				onkeydown={(e) => e.key === 'Escape' && q && (e.preventDefault(), (q = ''))}
			/>
			{#if q}
				<button class="leeren" type="button" title="Suche leeren" onclick={() => ((q = ''), feld?.focus())}>
					<IconMaterialSymbolsCloseRounded aria-hidden="true" /><span class="sr">Suche leeren</span>
				</button>
			{/if}
		</label>

		<div class="filter" role="radiogroup" aria-label="Herkunft">
			{#each Object.entries(HERKUNFT) as [k, name] (k)}
				<label class="pille" class:an={herkunft === k}>
					<input class="sr" type="radio" name="herkunft" value={k} bind:group={herkunft} />
					{#if k === 'hand'}<IconMaterialSymbolsBackHandOutlineRounded aria-hidden="true" />{/if}
					{#if k === 'api'}<IconMaterialSymbolsCableRounded aria-hidden="true" />{/if}
					{name}
					<span class="zahl">{zahl[/** @type {keyof typeof zahl} */ (k)]}</span>
				</label>
			{/each}
		</div>

		<div class="ordnen">
			<span class="anzahl" role="status">
				{#if live.notizen === null}
					&nbsp;
				{:else if sucht}
					{sichtbar.length === 1 ? 'ein Treffer' : `${sichtbar.length || 'keine'} Treffer`}
				{:else}
					{sichtbar.length === 1 ? 'eine Aufnahme' : `${sichtbar.length} Aufnahmen`}
				{/if}
			</span>
			<label class="sortieren">
				<span class="sr">Sortierung</span>
				<select class="feld" bind:value={sortierung} disabled={sucht} title={sucht ? 'Beim Suchen stehen die besten Treffer oben' : undefined}>
					{#each Object.entries(SORTIERUNG) as [k, name] (k)}<option value={k}>{sucht ? 'Beste Treffer' : name}</option>{/each}
				</select>
			</label>
		</div>
	</div>

	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div class="liste" onkeydown={taste} role="list" aria-label="Aufnahmen">
		{#if live.notizen === null}
			{#each [0, 1, 2] as i (i)}<div class="platzhalter" aria-hidden="true"><i></i><i></i><i></i></div>{/each}
		{:else if !alle.length}
			<p class="leer">
				Noch keine Aufnahmen. Unten startest du eine, oder ein Programm schickt Sprache über die API.
			</p>
		{:else if !sichtbar.length}
			<p class="leer">
				{#if sucht}
					Nichts gefunden für „{q}“{herkunft !== 'alle' ? ` unter „${HERKUNFT[herkunft]}“` : ''}.
				{:else}
					Keine Aufnahmen {herkunft === 'hand' ? 'von Hand' : 'per API'}.
				{/if}
				{#if herkunft !== 'alle'}
					<button class="link" type="button" onclick={() => (herkunft = 'alle')}>Alle zeigen</button>
				{/if}
			</p>
		{/if}

		{#each gruppen as g (g.titel)}
			{#if g.titel}<h3 class="tag">{g.titel}</h3>{/if}
			{#each g.notizen as n (n.id)}
				{@const hand = vonHand(n)}
				{@const v = vorschau(n)}
				<div role="listitem">
					<button
						type="button"
						class="eintrag"
						class:gewaehlt={n.id === gewaehlt && !verbinden}
						data-id={n.id}
						aria-current={n.id === gewaehlt && !verbinden ? 'true' : undefined}
						onclick={() => onwaehlen(n.id)}
					>
						<span class="kopf">
							<span class="herkunft" class:hand>
								{#if hand}<IconMaterialSymbolsBackHandOutlineRounded aria-hidden="true" />{:else}<IconMaterialSymbolsCableRounded
										aria-hidden="true"
									/>{/if}
								<span class="wer">{wer(n)}</span>
								{#if !hand}<span class="sr">, per API</span>{/if}
							</span>
							{#if !n.ended}
								<span class="laeuft"><span class="punkt" aria-hidden="true"></span>läuft</span>
							{:else if n.error}
								<span class="abbruch">abgebrochen</span>
							{:else if n.transient}
								<span class="fluechtig">nicht gespeichert</span>
							{/if}
							<span class="uhr">{sucht || sortierung === 'lang' ? `${tag(n.started, live.jetzt)}, ` : ''}{uhrzeit(n.started)}</span>
						</span>
						<span class="vorschau" class:still={!v.length}>
							{#if v.length}
								{#each v as s, i (i)}{#if s.an}<mark>{s.t}</mark>{:else}{s.t}{/if}{/each}
							{:else}
								{n.error ? 'Kein Text' : n.loading ? 'Lädt die Erkennung …' : n.waiting ? WARTET[n.source ?? 'mic'] : !n.ended ? 'Hört zu …' : 'Es wurde nichts erkannt.'}
							{/if}
						</span>
						<span class="fuss">
							<span class="dauer">{dauer(laenge(n, live.jetzt))}</span>
							{#if n.source}<span>{QUELLEN[n.source]}</span>{/if}
							{#if n.audio}<span class="gespeichert" title="Aufnahme gespeichert"
									><IconMaterialSymbolsSaveOutlineRounded aria-hidden="true" /><span class="sr">gespeichert</span></span
								>{/if}
						</span>
					</button>
				</div>
			{/each}
		{/each}
	</div>

	<button class="verbinden" class:gewaehlt={verbinden} type="button" aria-current={verbinden ? 'true' : undefined} onclick={onverbinden}>
		<IconMaterialSymbolsCableRounded aria-hidden="true" />
		<span>Programme verbinden</span>
		{#if live.abgelehnt.length}
			<span class="badge" title="{live.abgelehnt.length} Seiten wollten sich verbinden"
				>{live.abgelehnt.length}<span class="sr"> Seiten wollten sich verbinden</span></span
			>
		{/if}
	</button>
</aside>

<style>
	.leiste {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr) auto;
		min-height: 0;
		background: hsl(var(--color-bg-primary));
		border-right: 1px solid hsl(var(--color-border-primary));
	}

	.finden {
		display: grid;
		gap: 0.75rem;
		padding: 1rem 1rem 0.75rem;
		border-bottom: 1px solid hsl(var(--color-border-primary));
	}

	.suche {
		position: relative;
		display: block;
	}
	.suche > :global(svg) {
		position: absolute;
		left: 0.75rem;
		top: 50%;
		translate: 0 -50%;
		width: 1.375rem;
		height: 1.375rem;
		color: hsl(var(--color-text-secondary));
		pointer-events: none;
	}
	.suche input {
		width: 100%;
		padding-left: 2.5rem;
		padding-right: 2.75rem;
	}
	.suche input::-webkit-search-cancel-button {
		display: none;
	}
	.suche input:focus-visible {
		border-color: hsl(var(--color-border-focus));
		outline-offset: 0;
	}
	.leeren {
		position: absolute;
		right: 0.125rem;
		top: 50%;
		translate: 0 -50%;
		width: 2.5rem;
		height: 2.5rem;
		display: grid;
		place-items: center;
		border-radius: 50%;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
	}
	.leeren:hover {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	.leeren :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}

	/* Herkunft als Umschalter in einer Pille: gewählt in Tinte */
	.filter {
		display: grid;
		grid-template-columns: auto auto auto;
		gap: 2px;
		padding: 3px;
		border-radius: 2rem;
		background: hsl(var(--color-bg-secondary));
	}
	.pille {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.3125rem;
		min-height: 2.5rem;
		padding: 0 0.625rem;
		border-radius: 2rem;
		font-size: 0.9375rem;
		font-weight: 600;
		white-space: nowrap;
		cursor: pointer;
		user-select: none;
		transition:
			background-color 0.15s,
			color 0.15s;
	}
	.pille:hover:not(.an) {
		background: hsl(var(--color-bg-primary) / 0.7);
	}
	.pille.an {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	.pille:has(:focus-visible) {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.pille :global(svg) {
		flex: none;
		width: 1.125rem;
		height: 1.125rem;
	}
	.zahl {
		font-weight: 500;
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-secondary));
	}
	.pille.an .zahl {
		color: hsl(var(--color-neutral-10));
	}

	.ordnen {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}
	.anzahl {
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		font-variant-numeric: tabular-nums;
	}
	.sortieren select {
		border-color: transparent;
		background-color: transparent;
		font-weight: 600;
		font-size: 0.9375rem;
		padding-left: 0.625rem;
	}
	.sortieren select:hover:not(:disabled) {
		border-color: hsl(var(--color-border-secondary));
	}
	.sortieren select:disabled {
		cursor: default;
		background-image: none;
		padding-right: 0.625rem;
		color: hsl(var(--color-text-secondary));
	}

	/* relative: the absolutely placed .sr labels in the entries belong to the list and scroll with
	   it; otherwise they hang off the page and make the whole window scroll */
	.liste {
		position: relative;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 0.25rem 0.5rem 1rem;
		scrollbar-width: thin;
		scrollbar-color: hsl(var(--color-neutral-50) / 0.6) transparent;
	}
	.tag {
		position: sticky;
		top: 0;
		z-index: 1;
		padding: 0.875rem 0.625rem 0.375rem;
		background: hsl(var(--color-bg-primary));
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: hsl(var(--color-text-secondary));
	}

	.eintrag {
		display: grid;
		gap: 0.25rem;
		width: 100%;
		padding: 0.75rem 0.75rem 0.6875rem;
		margin-top: 2px;
		border-radius: 0.75rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.12s;
	}
	.eintrag:hover {
		background: hsl(var(--color-bg-secondary));
	}
	/* gewählt: Signal-Hauch wie die gewählte Aufnahme in der Bibliothek */
	.eintrag.gewaehlt {
		background: hsl(var(--color-green-10));
		box-shadow: inset 0 0 0 1px hsl(var(--color-green-200) / 0.35);
	}
	.eintrag:focus-visible {
		outline-offset: -2px;
	}

	.kopf {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
	}
	.herkunft {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-width: 0;
		font-weight: 700;
	}
	.herkunft :global(svg) {
		flex: none;
		width: 1.125rem;
		height: 1.125rem;
		color: hsl(var(--color-text-secondary));
	}
	.wer {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.uhr {
		margin-left: auto;
		flex: none;
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-tertiary));
	}
	.laeuft,
	.abbruch,
	.fluechtig {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		padding: 0 0.5rem;
		border-radius: 2rem;
		font-size: 0.8125rem;
		font-weight: 700;
		line-height: 1.5rem;
	}
	.laeuft {
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
	}
	.abbruch {
		background: hsl(var(--color-red-10));
		color: hsl(var(--color-text-error));
	}
	.fluechtig {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-secondary));
	}
	.punkt {
		width: 0.4375rem;
		height: 0.4375rem;
		border-radius: 50%;
		background: currentColor;
		animation: atmen 1.2s ease-in-out infinite;
	}
	@keyframes atmen {
		50% {
			opacity: 0.25;
		}
	}

	.vorschau {
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		overflow: hidden;
		font-size: 1rem;
		line-height: 1.45;
		color: hsl(var(--color-text-primary));
	}
	.vorschau.still {
		color: hsl(var(--color-text-tertiary));
		font-style: italic;
	}
	/* Treffer: der Textmarker des Editors */
	mark {
		background: hsl(var(--color-green-50));
		color: inherit;
		border-radius: 3px;
		box-shadow: 0 0 0 1px hsl(var(--color-green-50));
	}

	.fuss {
		display: flex;
		align-items: center;
		gap: 0.25rem 0.875rem;
		font-size: 0.875rem;
		color: hsl(var(--color-text-tertiary));
	}
	.dauer {
		font-variant-numeric: tabular-nums;
	}
	.gespeichert {
		display: inline-flex;
	}
	.gespeichert :global(svg) {
		width: 1rem;
		height: 1rem;
	}

	.leer {
		padding: 1.25rem 0.75rem;
		color: hsl(var(--color-text-secondary));
		text-wrap: pretty;
	}
	.link {
		display: inline;
		font-weight: 600;
		color: hsl(var(--color-text-primary));
		text-decoration: underline;
		text-underline-offset: 0.2em;
		text-decoration-color: hsl(var(--color-green-100));
		text-decoration-thickness: 2px;
		cursor: pointer;
	}

	/* Laden: drei ruhige Platzhalter in der Form eines Eintrags */
	.platzhalter {
		display: grid;
		gap: 0.5rem;
		padding: 0.875rem 0.75rem;
	}
	.platzhalter i {
		height: 0.875rem;
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-secondary));
	}
	.platzhalter i:first-child {
		width: 45%;
	}
	.platzhalter i:last-child {
		width: 70%;
	}

	.verbinden {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		min-height: 3.5rem;
		padding: 0 1.25rem;
		border-top: 1px solid hsl(var(--color-border-primary));
		font-weight: 600;
		cursor: pointer;
		text-align: left;
	}
	.verbinden:hover {
		background: hsl(var(--color-bg-secondary));
	}
	.verbinden.gewaehlt {
		background: hsl(var(--color-green-10));
	}
	.verbinden :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
	}
	.verbinden:focus-visible {
		outline-offset: -2px;
	}
	.badge {
		margin-left: auto;
		min-width: 1.5rem;
		height: 1.5rem;
		padding: 0 0.4rem;
		display: inline-grid;
		place-items: center;
		border-radius: 2rem;
		background: hsl(var(--color-green-100));
		color: hsl(var(--color-neutral-100));
		font-size: 0.875rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}

	@media (prefers-reduced-motion: reduce) {
		.punkt {
			animation: none;
		}
	}
</style>
