<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- „Meine Wörter“ unter „Meine Stimme“: Namen und Fachbegriffe, die der Live-Text so schreibt, wie sie
     hier stehen. Als Pillen in einem Feld wie die Sprachenwahl; Eingabe oder Verlassen des Felds nimmt das
     getippte Wort auf, das Kreuz an der Pille nimmt es wieder heraus. -->
<script>
	import { live } from './stand.svelte.js';

	const id = 'meine-woerter';
	let text = $state('');
	/** @type {HTMLInputElement | undefined} */
	let feld = $state();

	function dazu() {
		if (text.trim()) live.wortDazu(text);
		text = '';
	}

	/** @param {KeyboardEvent} e */
	function taste(e) {
		if (e.key === 'Enter') {
			e.preventDefault();
			dazu();
		}
	}
</script>

<div class="woerter">
	<label class="feldname" for={id}>Meine Wörter</label>
	<!-- ein Klick ins Feld, auch neben die Pillen, geht in die Eingabe -->
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="rahmen" onclick={() => feld?.focus()}>
		{#each live.woerter as w (w)}
			<span class="wort">
				{w}
				<button
					type="button"
					class="weg"
					aria-label="{w} entfernen"
					onpointerdown={(e) => e.preventDefault()}
					onclick={(e) => {
						e.stopPropagation();
						live.wortWeg(w);
					}}><IconMaterialSymbolsCloseRounded aria-hidden="true" /></button
				>
			</span>
		{/each}
		<input
			{id}
			bind:this={feld}
			bind:value={text}
			placeholder="+ Wort"
			maxlength="60"
			spellcheck="false"
			autocomplete="off"
			onkeydown={taste}
			onblur={dazu}
		/>
	</div>
	<span class="satz">
		Namen und Fachbegriffe, etwa „Moliri“: Der Live-Text schreibt sie so, auch wenn sie ein wenig anders klingen.
	</span>
</div>

<style>
	.woerter {
		display: grid;
		gap: 0.3rem;
	}
	/* wie die Sprachenwahl: ein Feld mit den Wörtern als Pillen */
	.rahmen {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0.3rem 0.5rem;
		border: 1px solid hsl(var(--color-border-secondary));
		border-radius: 0.5rem;
		background-color: hsl(var(--color-bg-primary));
		cursor: text;
		transition: border-color 0.15s;
	}
	.rahmen:hover {
		border-color: hsl(var(--color-text-secondary));
	}
	.rahmen:focus-within {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.wort {
		display: inline-flex;
		align-items: center;
		gap: 0.125rem;
		height: 2rem;
		max-width: 100%;
		padding: 0 0.25rem 0 0.75rem;
		border-radius: 999px;
		background: hsl(var(--color-bg-inverse));
		color: hsl(var(--color-text-inverse));
		font-size: 0.9375rem;
		font-weight: 600;
		white-space: nowrap;
	}
	.weg {
		flex: none;
		display: grid;
		place-items: center;
		width: 1.5rem;
		height: 1.5rem;
		border: 0;
		border-radius: 50%;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}
	.weg:hover {
		background: hsl(var(--color-text-inverse) / 0.2);
	}
	.weg :global(svg) {
		width: 1rem;
		height: 1rem;
		margin: 0;
	}
	input {
		flex: 1 1 6rem;
		min-width: 6rem;
		height: 2rem;
		border: 0;
		outline: 0;
		background: transparent;
		color: hsl(var(--color-text-primary));
		font: inherit;
		font-size: 1rem;
		font-weight: 400;
	}
	input::placeholder {
		color: hsl(var(--color-text-tertiary));
	}
	.satz {
		font-size: 0.9375rem;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
</style>
