<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Pager.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
	/**
	 * Voriger / nächster Eintrag mit Position („‹ 3 / 22 ›") — für Dialogköpfe, die durch eine
	 * Liste blättern, ohne sich zu schließen. Eine Stelle für `ui/Modal` (Kopf, `navigation`)
	 * und den Bild-Dialog (Reiterleiste von FileModalContent).
	 *
	 * `achse`: `liste` blättert senkrecht durch eine Tabelle (↑ ↓, Tastatur Alt+↑ / Alt+↓ im
	 * Modal), `bilder` waagerecht durch eine Bildfolge (← →).
	 * `position` 0 = der offene Eintrag steht nicht (mehr) in der Liste („– / 22").
	 */
	import * as m from '$lib/moliri/texte.js';

	/** @type {{
	 *   position: number,
	 *   gesamt: number,
	 *   zurueck?: (() => void) | null,
	 *   weiter?: (() => void) | null,
	 *   achse?: 'liste' | 'bilder'
	 * }} */
	let { position, gesamt, zurueck = null, weiter = null, achse = 'liste' } = $props();

	const liste = $derived(achse === 'liste');
</script>

<div class="blaetterer" role="group" aria-label={m.detail_blaettern()}>
	<button
		type="button"
		disabled={!zurueck}
		title={liste ? `${m.detail_vorher()} (Alt+↑)` : m.detail_vorher()}
		aria-label={m.detail_vorher()}
		aria-keyshortcuts={liste ? 'Alt+ArrowUp' : undefined}
		onclick={() => zurueck?.()}
	>
		{#if liste}<IconMaterialSymbolsKeyboardArrowUpRounded />{:else}<IconMaterialSymbolsChevronLeftRounded />{/if}
	</button>
	<span class="position" aria-live="polite">{position > 0 ? position : '–'} / {gesamt}</span>
	<button
		type="button"
		disabled={!weiter}
		title={liste ? `${m.detail_weiter()} (Alt+↓)` : m.detail_weiter()}
		aria-label={m.detail_weiter()}
		aria-keyshortcuts={liste ? 'Alt+ArrowDown' : undefined}
		onclick={() => weiter?.()}
	>
		{#if liste}<IconMaterialSymbolsKeyboardArrowDownRounded />{:else}<IconMaterialSymbolsChevronRightRounded />{/if}
	</button>
</div>

<style>
	.blaetterer {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 0.1rem;
	}
	/* Dieselben Icon-Knöpfe wie das ✕ im Modal-Kopf (`.modal-icon-button`). */
	button {
		width: 2.25rem;
		height: 2.25rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
		border: none;
		border-radius: 2rem;
		background: none;
		color: hsl(var(--color-text-primary));
		cursor: pointer;
		transition: background-color 150ms ease-out;
	}
	button :global(svg) {
		width: 20px;
		height: 20px;
	}
	button:hover:not(:disabled) {
		background: hsl(var(--color-interactive-hover));
	}
	button:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.position {
		min-width: 3.5rem;
		text-align: center;
		font-size: 0.75rem;
		font-variant-numeric: tabular-nums;
		color: hsl(var(--color-text-secondary));
	}
	@media (pointer: coarse) {
		button {
			width: 2.75rem;
			height: 2.75rem;
		}
	}
</style>
