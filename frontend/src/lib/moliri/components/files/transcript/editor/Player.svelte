<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Player.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Player.svelte). -->
<!-- Das Abspielen als Kopf des Editors, gleich über der Zeitleiste, die es bewegt: vorn Springen und
     Abspielen, daneben der Titel (in transcribe-go die Wahl der Aufnahme) und darunter die Zeit, wer
     gerade spricht, Schleife und Sprecherfilter; rechts Geschwindigkeit und Mitlaufen. Eine eigene
     Spur für die Stelle gibt es nicht: die Zeitleiste darunter ist sie. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { getLocale } from '$lib/moliri/texte.js';
	import { fmt, itemAt } from './model.js';
	import { spkColor } from './editor.svelte.js';

	/** title: neben dem Abspielknopf, über der Zeit
	 * @type {{ ed: import('./editor.svelte.js').Editor, title?: import('svelte').Snippet }} */
	let { ed, title } = $props();

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
</script>

<div class="player" role="group" aria-label={m.te_player()}>
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
	<div class="mitte">
		{#if title}<div class="titel">{@render title()}</div>{/if}
		<div class="jetzt">
			<span class="zeit"><b>{fmt(ed.time)}</b> / {fmt(d)}</span>
			{#if now != null}
				<span class="wer" style:--c={spkColor(now)}><span class="pkt" aria-hidden="true"></span>{ed.spkName(now)}</span>
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
		</div>
	</div>
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

<style>
	.player {
		display: flex;
		align-items: center;
		gap: 0.5rem 1rem;
		min-width: 0;
	}
	button {
		font: inherit;
		color: inherit;
		cursor: pointer;
	}
	.transport {
		flex: none;
		display: flex;
		align-items: center;
		gap: 0.125rem;
	}
	.rund {
		width: var(--hit);
		height: var(--hit);
		border: 0;
		border-radius: 50%;
		background: transparent;
		color: var(--ink-2);
		display: grid;
		place-items: center;
		padding: 0;
	}
	.rund:hover {
		background: var(--desk);
		color: var(--ink);
	}
	.rund :global(svg) {
		width: 1.625rem;
		height: 1.625rem;
		margin: 0;
	}
	/* der Abspielknopf: Signalgrün mit Tinte, wie „läuft“ in Moliri; ein feiner Rand hebt ihn vom Papier */
	.play {
		width: 3.25rem;
		height: 3.25rem;
		border: 0;
		border-radius: 50%;
		background: var(--signal);
		color: var(--ink);
		box-shadow: inset 0 0 0 1px hsl(var(--color-green-200) / 0.35);
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

	.mitte {
		flex: 1;
		min-width: 0;
		display: grid;
		gap: 0.125rem;
	}
	.titel {
		min-width: 0;
	}
	.jetzt {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.25rem 1rem;
		min-width: 0;
	}
	.zeit {
		font-size: 1.0625rem;
		font-variant-numeric: tabular-nums;
		color: var(--ink-2);
		white-space: nowrap;
	}
	.zeit b {
		color: var(--ink);
		font-weight: 600;
	}
	/* wer spricht: wie der Name in der Randspalte des Manuskripts */
	.wer {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--c);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.pkt {
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--c);
		flex: none;
	}

	/* Schleife und Sprecherfilter: erklären, warum das Abspielen springt */
	.filter {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		background: hsl(var(--color-green-10));
		color: var(--ink);
		border-radius: 2rem;
		padding: 0.125rem 0.125rem 0.125rem 0.875rem;
		font-size: 0.9375rem;
		font-weight: 600;
		white-space: nowrap;
	}
	.filter button {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		min-height: 2rem;
		border: 0;
		border-radius: 2rem;
		background: none;
		padding: 0 0.75rem 0 0.5rem;
		font-weight: 500;
	}
	.filter button:hover {
		background: hsl(var(--color-neutral-100) / 0.08);
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
		flex: none;
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
		border: 1px solid var(--rule-2);
		color: var(--ink-2);
	}
	.tempo:hover {
		border-color: var(--ink-3);
	}
	.tempo :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}
	.tempo select {
		appearance: none;
		border: 0;
		background: transparent
			url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%231b1c1d' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='m7 10 5 5 5-5'/%3E%3C/svg%3E")
			no-repeat right 0.25rem center / 1.125rem;
		color: var(--ink);
		font: inherit;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		padding: 0.5rem 1.75rem 0.5rem 0.25rem;
		cursor: pointer;
		border-radius: 1rem;
	}
	.mitlaufen {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1rem 0 0.75rem;
		border-radius: 2rem;
		border: 1px solid var(--rule-2);
		background: transparent;
		font-weight: 600;
		color: var(--ink-2);
	}
	.mitlaufen:hover {
		border-color: var(--ink-3);
		color: var(--ink);
	}
	/* gedrückt: Nebel mit Tintenrand, leiser als der Abspielknopf */
	.mitlaufen[aria-pressed='true'] {
		background: var(--desk);
		border-color: var(--ink);
		color: var(--ink);
	}
	.mitlaufen :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}

	/* im Fenster der Datei: kleiner */
	:global(.eng) .play {
		width: 2.75rem;
		height: 2.75rem;
	}
	:global(.eng) .play :global(svg) {
		width: 1.75rem;
		height: 1.75rem;
	}

	@media (max-width: 720px) {
		.player {
			flex-wrap: wrap;
		}
		.mitte {
			flex-basis: calc(100% - 11rem);
		}
		.rechts {
			margin-left: auto;
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
	}
</style>
