<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/TranscriptEditor.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/routes/editor/+page.svelte); in Moliri the body of the
     transcript tab. Der Editor: oben Stand des Speicherns, Suche, Rückgängig, Neu transkribieren,
     Export, Vollbild und Bedienung, darunter die Zeitleiste, dann Manuskript und Seitenspalte, unten
     der Player. Lesen, Abspielen, Korrigieren (Text, Bereiche, Zeitanker), Sprecher (Name, Sprache,
     Zusammenlegen) und Clips (Titel, Schlagworte, Abspielen, einzeln oder als Zusammenschnitt
     exportieren). Gespeichert wird von selbst; in Moliri wendet der Server Zusammenlegen gleich an und
     erkennt eine festgelegte Sprache von selbst neu. Kann der Speicher das nicht (transcribe-go, das
     diesen Editor spiegelt), steht oben „Neu berechnen“. Mit playerInHead (transcribe-go, wo der
     Editor die ganze Seite ist) steht das Abspielen als Tintenpult im Kopf, über der Zeitleiste, die
     es bewegt, und Suche, Rückgängig, Neu berechnen, Export, Vollbild und Bedienung als Werkzeugleiste
     unten. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { fmt, presetName, searchItems } from './model.js';
	import Zeitleiste from './Zeitleiste.svelte';
	import Manuskript from './Manuskript.svelte';
	import Seitenspalte from './Seitenspalte.svelte';
	import Player from './Player.svelte';
	import SprecherMenue from './SprecherMenue.svelte';
	import Bereich from './Bereich.svelte';
	import Clip from './Clip.svelte';

	/** start: open at a place (a fulltext match in Moliri): the search term goes into the search,
	 *  the match at the time is the one shown, the player stands there.
	 *  compact: little room (Moliri's file window), so a one-line head and a smaller timeline until
	 *  fullscreen; false where the editor is the whole page (transcribe-go).
	 *  title: shown above the figures in the head (transcribe-go: the choice of recording).
	 *  playerInHead: the player in the head, the tools in a bar at the bottom (transcribe-go); else
	 *  the tools in the head and the player as the bar at the bottom.
	 *  @type {{ ed: import('./editor.svelte.js').Editor, onrerun?: () => void, rerunOpen?: boolean, start?: { time: number | null, query: string } | null, compact?: boolean, title?: import('svelte').Snippet, playerInHead?: boolean }} */
	let { ed, onrerun, rerunOpen = false, start = null, compact = true, title, playerInHead = false } = $props();

	let help = $state(false);
	/** @type {HTMLButtonElement | undefined} */ let helpBtn = $state();
	let exportOpen = $state(false);
	/** @type {HTMLElement | undefined} */ let exportBox = $state();
	/** @type {HTMLElement | undefined} */ let root = $state();
	let fullscreen = $state(false);
	const eng = $derived(compact && !fullscreen);
	$effect.pre(() => {
		ed.compact = eng;
	});
	const FORMATS = $derived([
		['txt', m.te_export_txt(), m.te_export_txt_note()],
		['vtt', m.te_export_vtt(), m.te_export_vtt_note()],
		['html', m.te_export_html(), m.te_export_html_note()]
	]);

	// ---- Suche ----
	let query = $state('');
	let hitIndex = $state(0);
	const hits = $derived(searchItems(ed.items, query));
	$effect(() => {
		const list = hits;
		const set = new Set();
		for (const h of list) for (let i = h.from; i <= h.to; i++) set.add(i);
		ed.hits = set;
		ed.hitNow = list.length ? list[Math.min(hitIndex, list.length - 1)] : null;
	});
	// Opened at a fulltext match: once the text is there, search for the term and show the match at
	// (or after) the time; once the player is there, stand at the time without playing.
	let startPending = $state(Boolean(start));
	let seekPending = $state(start?.time != null);
	$effect(() => {
		if (!startPending || !ed.items.length) return;
		startPending = false;
		const q = String(start?.query ?? '').trim();
		if (!q) return;
		query = q;
		const t = start?.time;
		const list = searchItems(ed.items, q);
		const at = t == null ? -1 : list.findIndex((h) => (ed.items[h.to]?.end ?? 0) >= t - 0.05);
		hitIndex = Math.max(0, at);
	});
	$effect(() => {
		// `media` is no state; its metadata arriving (mediaDuration) says the player is there
		if (!seekPending || !ed.mediaDuration || !ed.media || start?.time == null) return;
		seekPending = false;
		ed.seek(start.time, false);
	});
	function moveHit(/** @type {number} */ step) {
		if (!hits.length) return;
		hitIndex = (hitIndex + step + hits.length) % hits.length;
	}
	function searchKey(/** @type {KeyboardEvent} */ e) {
		if (e.key === 'Enter') {
			e.preventDefault();
			moveHit(e.shiftKey ? -1 : 1);
		} else if (e.key === 'Escape' && query) {
			e.preventDefault();
			e.stopPropagation();
			query = '';
		}
	}

	async function toggleFullscreen() {
		if (document.fullscreenElement) await document.exitFullscreen();
		else await root?.requestFullscreen?.();
	}

	function keydown(/** @type {KeyboardEvent} */ e) {
		// nur, solange die Tastatur im Editor ist (er liegt in einem Fenster der Moliri-Seite)
		if (!root?.contains(/** @type {Node} */ (e.target)) && e.target !== document.body) return;
		if ((e.ctrlKey || e.metaKey) && e.code === 'Space') {
			e.preventDefault();
			return ed.toggle();
		}
		const t = /** @type {HTMLElement} */ (e.target);
		const undoKey = (e.ctrlKey || e.metaKey) && !e.shiftKey && e.key.toLowerCase() === 'z';
		// beim Tippen gehören die Tasten dem Text; Strg+Z erst, wenn keine Eingabe offen ist
		if (t.isContentEditable) {
			if (undoKey && !ed.editing) {
				e.preventDefault();
				ed.undo();
			}
			return;
		}
		if (['INPUT', 'SELECT', 'TEXTAREA'].includes(t.tagName)) return;
		// Strg+Z gehört dem Editor, auch ohne etwas zum Zurücknehmen: sonst nähme der Browser die
		// letzte Eingabe auf der Seite zurück (die Dateisuche, die dann das Fenster schließt)
		if (undoKey) {
			e.preventDefault();
			return ed.undo();
		}
		if (e.code === 'Space' && t.tagName !== 'BUTTON') {
			e.preventDefault();
			ed.toggle();
		} else if (e.code === 'ArrowLeft' && !e.altKey) ed.skip(-5);
		else if (e.code === 'ArrowRight' && !e.altKey) ed.skip(5);
		else if (e.key === '+') ed.zoom(0.5);
		else if (e.key === '-') ed.zoom(2);
		else if (e.key === 'Escape' && (ed.wordbar || ed.loop || help || exportOpen)) {
			// nur schließen, was offen ist; sonst schließt Esc das Fenster wie gewohnt
			e.stopPropagation();
			ed.wordbar = null;
			ed.loop = null;
			help = false;
			exportOpen = false;
		}
	}
</script>

<svelte:window
	onkeydown={keydown}
	onpagehide={() => ed.flush()}
	onbeforeunload={(e) => {
		if (ed.busy) e.preventDefault();
	}}
	onpointerdown={(e) => {
		const t = /** @type {HTMLElement} */ (e.target);
		if (help && !t.closest('.hilfe') && t !== helpBtn && !helpBtn?.contains(t)) help = false;
		if (exportOpen && !exportBox?.contains(t)) exportOpen = false;
	}}
/>
<svelte:document
	onfullscreenchange={() => {
		fullscreen = document.fullscreenElement === root;
	}}
/>

{#snippet tools()}
	<label class="suche">
		<IconMaterialSymbolsSearch aria-hidden="true" />
		<input type="search" placeholder={m.transcript_search()} aria-label={m.transcript_search()} bind:value={query} oninput={() => (hitIndex = 0)} onkeydown={searchKey} />
		{#if query.trim()}
			<span class="treffer" aria-live="polite">{hits.length ? m.transcript_search_hits({ current: hitIndex + 1, total: hits.length }) : m.transcript_search_none()}</span>
			<button type="button" class="pfeil" title={m.transcript_search_prev()} aria-label={m.transcript_search_prev()} disabled={!hits.length} onclick={() => moveHit(-1)}
				><IconMaterialSymbolsKeyboardArrowUpRounded /></button
			>
			<button type="button" class="pfeil" title={m.transcript_search_next()} aria-label={m.transcript_search_next()} disabled={!hits.length} onclick={() => moveHit(1)}
				><IconMaterialSymbolsKeyboardArrowDownRounded /></button
			>
		{/if}
	</label>
	{#if ed.canEdit}
		<button
			class="hilfe-btn"
			title={m.te_undo_hint()}
			disabled={!ed.canUndo && !ed.editing}
			onpointerdown={(e) => e.preventDefault()}
			onclick={() => ed.undo()}
		>
			<IconMaterialSymbolsUndoRounded /><span class="lbl">{m.te_undo()}</span>
		</button>
		{#if ed.recomputes}
			<!-- Sprache neu erkennen und Zusammenlegen rechnet der Server; alles andere wirkt sofort -->
			<button
				class="hilfe-btn rechnen"
				class:vorgemerkt={ed.pending}
				title={ed.pending ? m.te_recompute_due_hint() : m.te_recompute_hint()}
				disabled={!ed.pending || ed.computing}
				aria-busy={ed.computing}
				onclick={() => ed.recompute()}
			>
				{#if ed.computing}<IconEosIconsLoading /><span class="lbl">{m.te_recompute_busy()}</span>{:else}<IconMaterialSymbolsAutorenewRounded /><span class="lbl"
						>{m.te_recompute()}</span
					>{/if}
			</button>
		{/if}
		{#if onrerun}
			<button class="hilfe-btn" title={m.te_rerun_hint()} aria-expanded={rerunOpen} onclick={() => onrerun()}>
				<IconMaterialSymbolsAutorenewRounded /><span class="lbl">{m.te_rerun()}</span>
			</button>
		{/if}
	{/if}
	<div class="export" bind:this={exportBox}>
		<button
			class="hilfe-btn"
			aria-label={m.te_export_hint()}
			title={m.te_export_hint()}
			aria-expanded={exportOpen}
			aria-controls="export-formate"
			onclick={() => (exportOpen = !exportOpen)}
		>
			<IconMaterialSymbolsDownloadRounded /><span class="lbl">{m.transcript_export()}</span>
		</button>
		{#if exportOpen}
			<ul class="formate" id="export-formate" aria-label={m.te_formats()}>
				{#each FORMATS as [f, label, note] (f)}
					<li>
						<button
							type="button"
							onclick={() => {
								exportOpen = false;
								ed.exportTranscript(f);
							}}><b>{label}</b><span>{note}</span></button
						>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
	<button class="hilfe-btn rund" title={fullscreen ? m.te_fullscreen_exit() : m.te_fullscreen()} aria-label={fullscreen ? m.te_fullscreen_exit() : m.te_fullscreen()} aria-pressed={fullscreen} onclick={toggleFullscreen}>
		{#if fullscreen}<IconMaterialSymbolsCloseFullscreenRounded />{:else}<IconMaterialSymbolsOpenInFullRounded />{/if}
	</button>
	<button class="hilfe-btn rund" bind:this={helpBtn} title={m.te_help()} aria-label={m.te_help()} aria-expanded={help} aria-controls="hilfe" onclick={() => (help = !help)}>
		<IconMaterialSymbolsHelpOutlineRounded />
	</button>
{/snippet}

<div class="editor" class:vollbild={fullscreen} class:eng class:player-oben={playerInHead} bind:this={root}>
	<header class="kopf">
		<div class="titel">
			{@render title?.()}
			<div class="meta">
				<span>{m.te_length()} <b>{fmt(ed.final.duration)}</b></span>
				<span>{m.transcript_speakers()} <b>{ed.speakers.length}</b></span>
				{#if ed.final.preset}<span>{m.te_recognition()} <b>{presetName(ed.final.preset)}</b></span>{/if}
				<span>{m.te_corrections()} <b>{ed.nCorrections}</b></span>
				{#if ed.canEdit || ed.saveKind !== 'ok'}
					<span class="stand {ed.languagesBusy ? 'offen' : ed.saveKind}" role="status" aria-live="polite" title={ed.saveText}
						>{ed.languagesBusy && ed.saveKind === 'ok' ? m.te_languages_running() : ed.saveText}</span
					>
				{/if}
			</div>
		</div>
		{#if playerInHead}
			<Player {ed} inHead />
		{:else}
			{@render tools()}
		{/if}
	</header>

	<Zeitleiste {ed} />

	<main class="arbeit">
		<Manuskript {ed} />
		<Seitenspalte {ed} />
	</main>

	{#if playerInHead}
		<footer class="werkzeug">{@render tools()}</footer>
	{:else}
		<Player {ed} />
	{/if}
	<!-- schwebend, aber im Editor: sie brauchen dessen Farben (und erscheinen im Vollbild) -->
	<SprecherMenue {ed} />
	<Bereich {ed} />
	<Clip {ed} />

	{#if help}
		<div class="hilfe" id="hilfe" role="dialog" aria-label={m.te_help()}>
			<h2>{m.te_help()}</h2>
			<dl>
				{#if ed.canEdit}
					<dt>{m.te_help_text_t()}</dt>
					<dd>{m.te_help_text()}</dd>
					<dt>{m.te_help_range_t()}</dt>
					<dd>{m.te_help_range()}</dd>
					<dt>{m.te_help_anchor_t()}</dt>
					<dd>{m.te_help_anchor()}</dd>
					<dt>{m.te_clips()}</dt>
					<dd>{m.te_help_clips()}</dd>
				{/if}
				<dt>{m.transcript_export()}</dt>
				<dd>{m.te_help_export()}</dd>
				{#if ed.canEdit}
					<dt>{m.te_help_save_t()}</dt>
					<dd>{ed.recomputes ? m.te_help_save_recompute() : m.te_help_save()}</dd>
				{/if}
				<dt>{m.te_play()}</dt>
				<dd>{m.te_help_play()}</dd>
				<dt>{m.te_wordbar()}</dt>
				<dd>{m.te_help_wordbar()}</dd>
				<dt>{m.te_help_timeline_t()}</dt>
				<dd>{m.te_help_timeline()}</dd>
				<dt>{m.transcript_speakers()}</dt>
				<dd>{m.te_help_speakers()}</dd>
				<dt>{m.te_follow()}</dt>
				<dd>{m.te_help_follow()}</dd>
			</dl>
		</div>
	{/if}
</div>

<style>
	/* Farben des Editors aus den Moliri-Tokens; jede Farbe hat eine redaktionelle Bedeutung */
	.editor {
		--ink: hsl(var(--color-text-primary));
		--ink-2: hsl(var(--color-text-secondary));
		--ink-3: hsl(var(--color-text-tertiary));
		--future: hsl(var(--color-text-secondary));
		--paper: hsl(var(--color-bg-primary));
		--desk: hsl(var(--color-bg-secondary));
		--rule: hsl(var(--color-neutral-10));
		--rule-2: hsl(190 12% 80%);
		--head: hsl(var(--color-bg-inverse));
		--head-ink: hsl(var(--color-text-inverse));
		--head-ink-2: hsl(var(--color-neutral-10));
		--focus: hsl(var(--color-border-focus));
		/* Signalgrün: läuft gerade (Abspielknopf, Abspielkopf, das gesprochene Wort) */
		--signal: hsl(var(--color-green-100));
		--marker: hsl(var(--color-green-50));
		/* Redaktionsrot: nur die menschliche Hand */
		--red: hsl(var(--color-red-200));
		--red-soft: hsl(var(--color-red-10));
		--red-on-ink: hsl(var(--color-red-50));
		/* Regie-Blau: Musik, Applaus, neu erkannt */
		--blue: hsl(var(--color-blue-200));
		--blue-soft: hsl(var(--color-blue-10));
		--blue-on-ink: hsl(var(--color-blue-50));
		/* Zweifel-Ocker: die Maschine ist unsicher. Gelb statt Orange, damit Zweifel und Rotstift (die
		   Gewissheit der menschlichen Hand) nicht verwechselt werden; 3,1:1 auf Papier als Linie */
		--doubt: hsl(var(--color-yellow-200));
		--doubt-on-ink: hsl(var(--color-yellow-100));
		--fill: hsl(var(--color-text-secondary));
		--sel: hsl(var(--color-cyan-100) / 0.14);
		--wave: hsl(var(--color-neutral-50));
		--wave-past: hsl(var(--color-neutral-100));
		--font: var(--font-ui);
		/* Klickziele ab 44px, wie im Prototyp (PRODUCT.md von transcribe-go) */
		--hit: 44px;
		/* Sprecherfarben: kräftig, lesbar als Schrift auf Papier und unter weißer Schrift */
		--spk0: #0b6f78;
		--spk1: #6f42c1;
		--spk2: #9a5b00;
		--spk3: #2b7a4b;
		--spk4: #93398a;
		--spk5: #5f7012;
		--spk6: #8a5a3c;
		--spk7: #434b5c;
		--lang-en: #0b6f78;
		--lang-fr: #6f42c1;
		--lang-nl: #b85c00;
		--lang-it: #2b7a4b;
		--lang-es: #8a6d00;
		--lang-x: #5c677d;
		/* Clips sind Schnitte: Tinte, wie eine Klammer, die eine Redaktion über eine Passage zieht */
		--clip: hsl(var(--color-text-primary));
		--clip-ink: hsl(var(--color-text-inverse));

		flex: 1;
		height: 100%;
		min-height: 0;
		width: 100%;
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		grid-template-rows: auto auto minmax(0, 1fr) auto;
		background: var(--desk);
		color: var(--ink);
		font-family: var(--font-ui);
		line-height: 1.6;
	}
	.editor.vollbild {
		height: 100vh;
	}
	.editor :global(::selection) {
		background: var(--marker);
		color: var(--ink);
	}

	/* Felder wie im Prototyp (dort app.scss): Papier, Schieferrahmen, Klickhöhe 44px */
	.editor :global(.feld) {
		height: var(--hit);
		padding: 0 0.75rem;
		border: 1px solid hsl(var(--color-border-secondary, var(--color-border-primary)));
		border-radius: 0.5rem;
		background-color: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		font: inherit;
		font-size: 1rem;
		transition: border-color 0.15s;
	}
	.editor :global(.feld:hover) {
		border-color: hsl(var(--color-text-secondary));
	}
	.editor :global(select.feld) {
		appearance: none;
		height: auto;
		min-height: var(--hit);
		padding-block: 0.55rem;
		line-height: 1.4;
		padding-right: 2.25rem;
		cursor: pointer;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'%3E%3Cpath fill='%231b1c1d' d='M12 15.4 6.6 10l1.4-1.4 4 4 4-4 1.4 1.4z'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 0.6rem center;
		background-size: 1.25rem;
	}
	.editor :global(.feldname) {
		display: grid;
		gap: 0.3rem;
		font-size: 0.9375rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
	}
	.editor :global(input[type='checkbox']) {
		appearance: auto;
		accent-color: hsl(var(--color-neutral-100));
		cursor: pointer;
	}

	.kopf {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1rem 0.5rem 1.25rem;
		background: var(--paper);
		border-bottom: 1px solid var(--rule);
	}
	.titel {
		min-width: 0;
		flex: 1;
	}
	.meta {
		min-width: 0;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 1.25rem;
		font-size: 0.9375rem;
		color: var(--ink-2);
	}
	.meta b {
		color: var(--ink);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	/* eine lange Meldung bleibt auf einer Zeile, der Kopf springt nicht */
	.stand {
		display: inline-block;
		flex: 1 1 9rem;
		min-width: 0;
		max-width: 26rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.stand::before {
		content: '';
		display: inline-block;
		vertical-align: 0.0625rem;
		margin-right: 0.375rem;
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: var(--signal);
		box-shadow: 0 0 0 1px hsl(var(--color-green-200) / 0.5);
		transition: background-color 0.2s;
	}
	/* wartet aufs Speichern, eine Eingabe ist offen oder der Server erkennt neu: ein leerer Ring */
	.stand.offen::before {
		background: transparent;
		box-shadow: inset 0 0 0 2px var(--ink-3);
	}
	/* nicht gespeichert: Zweifel-Ocker (Rot bleibt der menschlichen Hand vorbehalten) */
	.stand.fehler {
		color: var(--doubt);
		font-weight: 600;
	}
	.stand.fehler::before {
		background: var(--doubt);
		box-shadow: none;
	}

	.suche {
		flex: 0 1 17rem;
		min-width: 9rem;
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 0.375rem 0 0.75rem;
		border: 1px solid var(--rule-2);
		border-radius: 2rem;
		background: var(--paper);
		color: var(--ink-3);
	}
	.suche:focus-within {
		border-color: var(--focus);
	}
	.suche :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		flex: none;
	}
	.suche input {
		flex: 1;
		min-width: 0;
		border: 0;
		outline: none;
		background: transparent;
		font: inherit;
		color: var(--ink);
	}
	.treffer {
		flex: none;
		font-size: 0.875rem;
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}
	.pfeil {
		flex: none;
		width: 2rem;
		height: 2rem;
		display: grid;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 50%;
		background: none;
		color: var(--ink-2);
		cursor: pointer;
	}
	.pfeil:hover:not(:disabled) {
		background: var(--desk);
		color: var(--ink);
	}
	.pfeil:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.hilfe-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1.125rem 0 0.875rem;
		border: 2px solid var(--ink);
		border-radius: 2rem;
		background: var(--paper);
		color: var(--ink);
		font: inherit;
		font-weight: 700;
		cursor: pointer;
		flex: none;
	}
	.hilfe-btn.rund {
		width: var(--hit);
		padding: 0;
		justify-content: center;
	}
	.hilfe-btn:hover:not(:disabled),
	.hilfe-btn[aria-expanded='true'],
	.hilfe-btn[aria-pressed='true'] {
		background: var(--ink);
		color: var(--paper);
	}
	.hilfe-btn:disabled {
		border-color: var(--rule-2);
		color: var(--ink-3);
		cursor: default;
	}
	/* etwas ist vorgemerkt, das nur der Server rechnen kann: der Knopf wird zur Tintenpille */
	.rechnen.vorgemerkt:not(:disabled) {
		background: var(--ink);
		color: var(--paper);
	}
	.rechnen.vorgemerkt:not(:disabled):hover {
		background: color-mix(in oklab, var(--ink) 80%, var(--paper));
		border-color: color-mix(in oklab, var(--ink) 80%, var(--paper));
	}
	/* rechnet: läuft gerade, also Signalgrün auf Tinte */
	.rechnen[aria-busy='true'] {
		border-color: var(--ink);
		background: var(--ink);
		color: var(--paper);
	}
	.rechnen[aria-busy='true'] :global(svg) {
		color: var(--signal);
	}
	.hilfe-btn :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}

	.export {
		position: relative;
		flex: none;
	}
	.formate {
		position: absolute;
		right: 0;
		top: calc(100% + 0.5rem);
		z-index: 46;
		width: 19rem;
		margin: 0;
		padding: 0.375rem;
		list-style: none;
		background: var(--paper);
		border-radius: 1rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	.formate button {
		display: grid;
		gap: 0.125rem;
		width: 100%;
		min-height: var(--hit);
		padding: 0.5rem 0.875rem;
		border: 0;
		border-radius: 0.75rem;
		background: none;
		color: var(--ink);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.formate button:hover {
		background: var(--desk);
	}
	.formate b {
		font-weight: 600;
	}
	.formate span {
		font-size: 0.875rem;
		color: var(--ink-3);
	}

	/* Player im Kopf: links Titel und Stand, rechts das Tintenpult, das bei wenig Breite darunter rückt */
	.player-oben .kopf {
		flex-wrap: wrap;
		gap: 0.5rem 1.5rem;
	}
	.player-oben .titel {
		flex: 0 1 30rem;
	}
	/* die Werkzeuge unten: Papier mit Haarlinie, die Suche vorn und breit, Export und Bedienung rechts */
	.werkzeug {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1rem 0.5rem 1.25rem;
		background: var(--paper);
		border-top: 1px solid var(--rule);
		position: relative;
		z-index: 5;
	}
	.werkzeug .suche {
		flex: 0 1 26rem;
	}
	.werkzeug .export {
		margin-left: auto;
	}
	/* die Formate gehen nach oben auf, über der Leiste */
	.werkzeug .formate {
		top: auto;
		bottom: calc(100% + 0.5rem);
	}
	.player-oben .hilfe {
		top: auto;
		bottom: 4.5rem;
	}

	.arbeit {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 21.25rem;
		grid-template-rows: minmax(0, 1fr);
		min-height: 0;
	}

	.hilfe {
		position: absolute;
		right: 1.25rem;
		top: 4rem;
		width: 26rem;
		max-width: calc(100% - 2.5rem);
		max-height: calc(100% - 8rem);
		overflow-y: auto;
		z-index: 45;
		background: var(--paper);
		color: hsl(var(--color-text-secondary));
		border-radius: 1rem;
		padding: 1.25rem 1.375rem;
		font-size: 0.9375rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	.editor {
		position: relative;
	}
	.hilfe h2 {
		font-size: 1.0625rem;
		color: hsl(var(--color-text-primary));
		margin-bottom: 0.625rem;
	}
	.hilfe dl {
		display: grid;
		grid-template-columns: 7.5rem 1fr;
		gap: 0.5rem 0.875rem;
		margin: 0;
	}
	.hilfe dt {
		color: hsl(var(--color-text-primary));
		font-weight: 600;
	}
	.hilfe dd {
		margin: 0;
		line-height: 1.5;
	}

	/* im Fenster der Datei: Kopf auf einer Zeile, Knöpfe als Symbole (Name im Titel), etwas kleiner */
	.eng {
		--hit: 38px;
	}
	.eng .kopf {
		padding: 0.375rem 0.75rem;
		gap: 0.5rem;
	}
	.eng .meta {
		font-size: 0.875rem;
		gap: 0.125rem 0.875rem;
	}
	.eng .hilfe-btn {
		width: var(--hit);
		padding: 0;
		justify-content: center;
		border-width: 1.5px;
	}
	.eng .hilfe-btn .lbl {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
	}
	@media (max-width: 1180px) {
		.arbeit {
			grid-template-columns: 1fr;
			grid-template-rows: minmax(0, 1fr) auto;
		}
		/* im Kopf wird es eng; die Leiste unten hat Platz für die Namen bis 720px */
		.kopf .hilfe-btn:not(.rund) {
			width: var(--hit);
			padding: 0;
			justify-content: center;
		}
		.kopf .hilfe-btn .lbl {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip: rect(0 0 0 0);
		}
	}
	@media (max-width: 720px) {
		.kopf,
		.werkzeug {
			flex-wrap: wrap;
			padding: 0.5rem 0.75rem;
		}
		.werkzeug .suche {
			flex: 1 1 100%;
		}
		.werkzeug .hilfe-btn:not(.rund) {
			width: var(--hit);
			padding: 0;
			justify-content: center;
		}
		.werkzeug .hilfe-btn .lbl {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip: rect(0 0 0 0);
		}
		.meta {
			font-size: 0.875rem;
			gap: 0.125rem 0.75rem;
		}
		.hilfe dl {
			grid-template-columns: 1fr;
		}
		.hilfe dd {
			margin-bottom: 0.5rem;
		}
	}
</style>
