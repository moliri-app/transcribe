<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Player.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Player.svelte). -->
<!-- Der Player als feste Leiste am unteren Rand, wie bei einem Musikspieler: oben die Stelle in der
     Aufnahme als Schieberegler, darunter Springen, Abspielen, Zeit, wer gerade spricht, der
     Sprecherfilter und rechts Geschwindigkeit und Mitlaufen. Mit inHead steht er als Tintenpult im
     Kopf des Editors, alles auf einer Zeile: Springen und Abspielen, die Zeit, die Stelle als Spur
     zwischen gespielter und ganzer Länge, wer spricht, Tempo und Mitlaufen. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { getLocale } from '$lib/moliri/texte.js';
	import { fmt, itemAt } from './model.js';
	import { spkColor } from './editor.svelte.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor, inHead?: boolean }} */
	let { ed, inHead = false } = $props();

	const RATES = [0.75, 1, 1.25, 1.5, 2].map((v) => [v, `${v.toLocaleString(getLocale())}×`]);

	/** wer an der Stelle des Abspielkopfs spricht (oder zuletzt sprach) */
	const now = $derived.by(() => {
		const i = itemAt(ed.items, ed.time);
		for (let k = i; k >= 0 && k > i - 40; k--) {
			const it = ed.items[k];
			if (it.kind === 'w' && it.speaker != null) return it.speaker;
		}
		return null;
	});
	const d = $derived(ed.duration());
	const pct = $derived(Math.min(100, (ed.time / d) * 100));
</script>

{#snippet stelle()}
	<input
		class="stelle"
		type="range"
		min="0"
		max={d}
		step="0.1"
		value={ed.time}
		style:--pct="{pct}%"
		aria-label={m.te_player_position()}
		aria-valuetext={m.te_player_of({ time: fmt(ed.time), total: fmt(d) })}
		oninput={(e) => ed.seek(+e.currentTarget.value, false)}
	/>
{/snippet}

<div class="player" class:kopf={inHead} role="group" aria-label={m.te_player()}>
	{#if !inHead}{@render stelle()}{/if}
	<div class="reihe">
		<div class="transport">
			<button class="rund" aria-label={m.te_player_back()} title="{m.te_player_back()} (←)" onclick={() => ed.skip(-5)}>
				<IconMaterialSymbolsReplay5Rounded />
			</button>
			<button class="play" aria-label={ed.playing ? m.te_player_pause() : m.te_player_play()} onclick={() => ed.toggle()}>
				{#if ed.playing}<IconMaterialSymbolsPauseRounded />{:else}<IconMaterialSymbolsPlayArrowRounded />{/if}
			</button>
			<button class="rund" aria-label={m.te_player_forward()} title="{m.te_player_forward()} (→)" onclick={() => ed.skip(5)}>
				<IconMaterialSymbolsForward5Rounded />
			</button>
		</div>
		{#if inHead}
			<div class="spur">
				<span class="zeit"><b>{fmt(ed.time)}</b></span>
				{@render stelle()}
				<span class="zeit">{fmt(d)}</span>
			</div>
			<span class="wer" class:leer={now == null} style:--c={now == null ? 'transparent' : spkColor(now)}
				><span class="pkt" aria-hidden="true"></span>{now == null ? '' : ed.spkName(now)}</span
			>
		{:else}
			<div class="jetzt">
				<span class="zeit"><b>{fmt(ed.time)}</b> / {fmt(d)}</span>
				{#if now != null}
					<span class="wer" style:--c={spkColor(now)}><span class="pkt" aria-hidden="true"></span>{ed.spkName(now)}</span>
				{/if}
			</div>
		{/if}
		{#if ed.loop}
			<span class="filter">
				<IconMaterialSymbolsRepeatRounded />
				<span>{m.te_loop()} <span class="zahl">{fmt(ed.loop.a)}–{fmt(ed.loop.b)}</span></span>
				<button title="{m.te_loop_end()} (Esc)" onclick={() => (ed.loop = null)}><IconMaterialSymbolsCloseRounded />{m.te_loop_end_short()}</button>
			</span>
		{/if}
		{#if ed.filterLabel}
			<span class="filter">
				<span>{ed.filterLabel}</span>
				<button title={m.te_show_all_speakers()} onclick={() => ed.showAll()}><IconMaterialSymbolsCloseRounded />{m.te_show_all_short()}</button>
			</span>
		{/if}
		<div class="rechts">
			<label class="tempo">
				<IconMaterialSymbolsSpeedRounded />
				<select aria-label={m.te_player_speed()} value={String(ed.rate)} onchange={(e) => ed.setRate(+e.currentTarget.value)}>
					{#each RATES as [v, l] (v)}<option value={String(v)}>{l}</option>{/each}
				</select>
			</label>
			<button class="mitlaufen" aria-pressed={ed.follow} title={m.te_follow_hint()} onclick={() => (ed.follow = !ed.follow)}>
				<IconMaterialSymbolsVerticalAlignCenterRounded /><span class="lbl">{m.te_follow()}</span>
			</button>
		</div>
	</div>
</div>

<style>
	.player {
		background: var(--head);
		color: var(--head-ink);
		position: relative;
		z-index: 5;
	}
	/* die Stelle: eine dünne Spur über die ganze Breite, gespielt in Signalgrün */
	.stelle {
		appearance: none;
		display: block;
		width: 100%;
		height: 6px;
		margin: 0;
		background: linear-gradient(to right, var(--signal) var(--pct), hsl(var(--color-neutral-0) / 0.16) var(--pct));
		cursor: pointer;
		transition: height 0.15s;
	}
	.stelle:hover,
	.stelle:focus-visible {
		height: 10px;
	}
	.stelle:focus-visible {
		outline: 2px solid var(--signal);
		outline-offset: 0;
	}
	.stelle::-webkit-slider-thumb {
		appearance: none;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--signal);
		border: 2px solid var(--head);
		opacity: 0;
		transition: opacity 0.15s;
	}
	.stelle:hover::-webkit-slider-thumb,
	.stelle:focus-visible::-webkit-slider-thumb {
		opacity: 1;
	}
	.stelle::-moz-range-thumb {
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--signal);
		border: 2px solid var(--head);
	}

	.reihe {
		display: flex;
		align-items: center;
		gap: 0.75rem 1.5rem;
		padding: 0.375rem 1.25rem 0.5rem;
		min-height: 4rem;
	}
	.transport {
		display: flex;
		align-items: center;
		gap: 0.375rem;
	}
	button {
		font: inherit;
		color: inherit;
		cursor: pointer;
	}
	.rund {
		width: var(--hit);
		height: var(--hit);
		border: 0;
		border-radius: 50%;
		background: transparent;
		display: grid;
		place-items: center;
		padding: 0;
	}
	.rund:hover {
		background: hsl(var(--color-neutral-0) / 0.1);
	}
	.rund :global(svg) {
		width: 1.625rem;
		height: 1.625rem;
		margin: 0;
	}
	/* der Abspielknopf: Signalgrün mit Tinte, wie „läuft“ in Moliri */
	.play {
		width: 3.25rem;
		height: 3.25rem;
		border: 0;
		border-radius: 50%;
		background: var(--signal);
		color: var(--head);
		display: grid;
		place-items: center;
		padding: 0;
		transition:
			transform 0.15s,
			filter 0.15s;
	}
	.play:hover {
		transform: scale(1.05);
		filter: brightness(1.05);
	}
	.play :global(svg) {
		width: 2rem;
		height: 2rem;
		margin: 0;
	}
	.player :global(:focus-visible) {
		outline-color: var(--signal);
	}

	.jetzt {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
		min-width: 0;
	}
	.zeit {
		font-size: 1.0625rem;
		font-variant-numeric: tabular-nums;
		color: var(--head-ink-2);
		white-space: nowrap;
	}
	.zeit b {
		color: var(--head-ink);
		font-weight: 600;
	}
	.wer {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.875rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--head-ink);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.pkt {
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--c);
		box-shadow: 0 0 0 1.5px hsl(var(--color-neutral-0) / 0.75);
		flex: none;
	}

	/* Sprecherfilter: erklärt, warum das Abspielen springt */
	.filter {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		background: hsl(var(--color-green-100) / 0.14);
		color: var(--signal);
		border-radius: 2rem;
		padding: 0.25rem 0.25rem 0.25rem 1rem;
		font-weight: 600;
		white-space: nowrap;
	}
	.filter button {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		min-height: 2.25rem;
		border: 0;
		border-radius: 2rem;
		background: none;
		padding: 0 0.75rem 0 0.5rem;
		font-weight: 500;
		color: var(--head-ink);
	}
	.filter button:hover {
		background: hsl(var(--color-neutral-0) / 0.12);
	}
	.filter > :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0 0.125rem 0 -0.375rem;
	}
	.zahl {
		font-variant-numeric: tabular-nums;
	}
	.filter :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
		margin: 0;
	}

	.rechts {
		margin-left: auto;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.tempo {
		display: inline-flex;
		align-items: center;
		gap: 0.125rem;
		min-height: var(--hit);
		padding: 0 0.375rem 0 0.75rem;
		border-radius: 2rem;
		border: 1px solid hsl(var(--color-neutral-0) / 0.22);
		color: var(--head-ink-2);
	}
	.tempo :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}
	.tempo select {
		appearance: none;
		border: 0;
		background: transparent url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23e8edeb' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='m7 10 5 5 5-5'/%3E%3C/svg%3E")
			no-repeat right 0.25rem center / 1.125rem;
		color: var(--head-ink);
		font: inherit;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		padding: 0.5rem 1.75rem 0.5rem 0.25rem;
		cursor: pointer;
		border-radius: 1rem;
	}
	.tempo select option {
		color: hsl(var(--color-neutral-100));
		background: hsl(var(--color-neutral-0));
	}
	.tempo:hover {
		border-color: hsl(var(--color-neutral-0) / 0.4);
	}
	.mitlaufen {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1rem 0 0.75rem;
		border-radius: 2rem;
		border: 1px solid hsl(var(--color-neutral-0) / 0.22);
		background: transparent;
		font-weight: 600;
		color: var(--head-ink-2);
	}
	.mitlaufen:hover {
		background: hsl(var(--color-neutral-0) / 0.08);
	}
	/* gedrückt: Papier auf Tinte, wie der aktive Eintrag der App-Leiste */
	.mitlaufen[aria-pressed='true'] {
		background: var(--paper);
		border-color: var(--paper);
		color: var(--head);
	}
	.mitlaufen :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}

	/* im Kopf: ein Pult aus Tinte, eine Zeile; die Stelle ist eine gerundete Spur zwischen den Zeiten */
	.player.kopf {
		flex: 1 1 36rem;
		min-width: 0;
		border-radius: 2.25rem;
		z-index: 6;
	}
	.kopf .reihe {
		gap: 0.5rem 1rem;
		min-height: 0;
		padding: 0.25rem 0.5rem 0.25rem 0.25rem;
	}
	.kopf .transport {
		gap: 0.125rem;
	}
	.spur {
		flex: 1 1 10rem;
		min-width: 8rem;
		display: flex;
		align-items: center;
		gap: 0.625rem;
	}
	.kopf .stelle {
		flex: 1;
		min-width: 0;
		width: auto;
		border-radius: 3px;
	}
	.kopf .stelle::-webkit-slider-thumb {
		opacity: 1;
	}
	.kopf .wer {
		flex: 0 1 11rem;
		min-width: 0;
	}
	.kopf .wer.leer {
		visibility: hidden;
	}

	@media (max-width: 720px) {
		.reihe {
			flex-wrap: wrap;
			gap: 0.5rem 0.75rem;
			padding: 0.375rem 0.75rem 0.5rem;
		}
		.play {
			width: 3rem;
			height: 3rem;
		}
		.mitlaufen .lbl {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip: rect(0 0 0 0);
		}
		.mitlaufen {
			padding: 0 0.625rem;
		}
		.filter {
			order: 5;
		}
	}
</style>
