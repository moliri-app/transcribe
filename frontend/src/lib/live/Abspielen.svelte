<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Die gespeicherte Datei einer Live-Aufnahme abspielen: Abspielknopf, Stelle als Schieberegler,
     Zeit und Länge, im Look der Player-Leiste des Editors, aber als Band im Kopf der Aufnahme.
     Die Aufnahme liest die Zeit, um den Satz zu markieren, der gerade läuft, und springt mit `ab`
     zu einem Satz. Die Bibliothek spielt damit jede Aufnahme an (ihre Abspielleiste und die Details
     einer Datei). -->
<script>
	import { dauer } from '$lib/api.js';

	/** datei: der Name unter /audio/; titel: wie die Seite sie nennt (sonst datei); zeit: die Stelle in
	 * Sekunden; gespielt: es lief schon einmal (dann zeigt die Aufnahme den Satz dazu); pause: steht
	 * gerade; sofort: beginnt gleich zu spielen
	 * @type {{ datei: string, titel?: string, zeit?: number, gespielt?: boolean, pause?: boolean, sofort?: boolean }} */
	let { datei, titel = datei, zeit = $bindable(0), gespielt = $bindable(false), pause = $bindable(true), sofort = false } = $props();

	/** @type {HTMLAudioElement | undefined} */
	let el = $state();
	let laenge = $state(0);
	/** warum sie sich nicht abspielen lässt: die Datei ist fort, oder ihr Format kennt der Browser nicht */
	let fehlt = $state(/** @type {'' | 'fort' | 'format'} */ (''));

	// eine andere Datei beginnt von vorn
	$effect.pre(() => {
		datei;
		fehlt = '';
		gespielt = false;
		zeit = laenge = 0;
	});

	/** Abspielen ging nicht: liegt die Datei noch da, kann der Browser ihr Format nicht lesen. */
	async function fehler() {
		const r = await fetch(`/audio/${encodeURIComponent(datei)}`, { method: 'HEAD' }).catch(() => null);
		fehlt = r?.ok ? 'format' : 'fort';
	}

	$effect(() => {
		if (!pause) gespielt = true;
	});

	const pct = $derived(laenge ? Math.min(100, (zeit / laenge) * 100) : 0);

	/** spielt ab der Stelle t (Sekunden)
	 * @param {number} t */
	export function ab(t) {
		if (!el) return;
		el.currentTime = t;
		el.play();
	}

	export function umschalten() {
		if (el?.paused) el.play();
		else el?.pause();
	}
</script>

{#if fehlt === 'format'}
	<p class="fehlt">
		<IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" />„{titel}“ lässt sich hier nicht anhören: Dieses Format kann
		das Fenster nicht abspielen. Transkribieren geht trotzdem.
	</p>
{:else if fehlt}
	<p class="fehlt">
		<IconMaterialSymbolsInfoOutlineRounded aria-hidden="true" />„{titel}“ lässt sich nicht abspielen. Wurde die Datei
		verschoben oder gelöscht?
	</p>
{:else}
	<div class="spieler" role="group" aria-label="Aufnahme abspielen">
		<audio
			bind:this={el}
			src="/audio/{encodeURIComponent(datei)}"
			preload="metadata"
			autoplay={sofort}
			bind:currentTime={zeit}
			bind:duration={laenge}
			bind:paused={pause}
			onerror={fehler}
		></audio>
		<button class="play" type="button" aria-label={pause ? 'Abspielen' : 'Anhalten'} onclick={umschalten}>
			{#if pause}<IconMaterialSymbolsPlayArrowRounded aria-hidden="true" />{:else}<IconMaterialSymbolsPauseRounded
					aria-hidden="true"
				/>{/if}
		</button>
		<span class="zeit">{dauer(zeit)}</span>
		<input
			class="stelle"
			type="range"
			min="0"
			max={laenge || 0}
			step="0.1"
			value={zeit}
			style:--pct="{pct}%"
			aria-label="Stelle in der Aufnahme"
			aria-valuetext="{dauer(zeit)} von {dauer(laenge)}"
			oninput={(e) => el && (el.currentTime = +e.currentTarget.value)}
		/>
		<span class="zeit">{dauer(laenge)}</span>
	</div>
{/if}

<style>
	.spieler {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		max-width: 40rem;
	}
	/* Abspielen: Tintenkreis wie in der Player-Leiste, in Papier gedreht */
	.play {
		flex: none;
		width: 2.75rem;
		height: 2.75rem;
		display: grid;
		place-items: center;
		border-radius: 50%;
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
		cursor: pointer;
		transition: transform 0.15s;
	}
	.play:hover {
		transform: scale(1.05);
	}
	.play :global(svg) {
		width: 1.75rem;
		height: 1.75rem;
	}
	.zeit {
		flex: none;
		min-width: 2.75rem;
		font-size: 0.9375rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-secondary));
	}
	.zeit:last-child {
		text-align: right;
	}

	/* die Stelle: gespielter Teil in Tinte, der Rest als Haarlinie */
	.stelle {
		flex: 1;
		min-width: 4rem;
		height: var(--hit);
		margin: 0;
		background: transparent;
		appearance: none;
		cursor: pointer;
	}
	.stelle::-webkit-slider-runnable-track {
		height: 0.375rem;
		border-radius: 2rem;
		background: linear-gradient(
			to right,
			hsl(var(--color-neutral-100)) var(--pct),
			hsl(var(--color-border-primary)) var(--pct)
		);
	}
	.stelle::-moz-range-track {
		height: 0.375rem;
		border-radius: 2rem;
		background: linear-gradient(
			to right,
			hsl(var(--color-neutral-100)) var(--pct),
			hsl(var(--color-border-primary)) var(--pct)
		);
	}
	.stelle::-webkit-slider-thumb {
		appearance: none;
		width: 1rem;
		height: 1rem;
		margin-top: -0.3125rem;
		border-radius: 50%;
		background: hsl(var(--color-neutral-100));
		border: 2px solid hsl(var(--color-bg-primary));
	}
	.stelle::-moz-range-thumb {
		width: 1rem;
		height: 1rem;
		border-radius: 50%;
		background: hsl(var(--color-neutral-100));
		border: 2px solid hsl(var(--color-bg-primary));
	}

	.fehlt {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
	}
	.fehlt :global(svg) {
		flex: none;
		width: 1.25rem;
		height: 1.25rem;
		margin-top: 0.125rem;
	}
</style>
