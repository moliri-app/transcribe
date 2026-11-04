<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Tabs.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
	/**
	 * Lightweight tab strip. The body is a snippet that receives the active tab id,
	 * so the parent decides what each tab renders.
	 *
	 * Form nach DESIGN.md („Reiter"): Wörter in Label-Typo auf dem Grund der Fläche, der
	 * aktive Reiter ist eine Tintenfläche mit 6px Radius. Viele Reiter (/layouts) scrollen
	 * waagerecht statt übers Blatt zu laufen. Ohne `children` schaltet die Leiste nur
	 * etwas darunter um — dann gibt es keinen (leeren) Inhaltsbereich.
	 *
	 * @typedef {{ id: string, label: string, disabled?: boolean }} Tab
	 */

	/** @type {{ tabs?: Tab[], activeTab?: string, children?: import('svelte').Snippet<[string]>, onChange?: (id: string) => void }} */
	let {
		/** @type {Tab[]} */
		tabs = [],
		/** the currently active tab id (bindable) @type {string} */
		activeTab = $bindable(''),
		/** @type {import('svelte').Snippet<[string]> | undefined} */
		children,
		/** @type {((id: string) => void) | undefined} */
		onChange = undefined,
	} = $props();

	// Fall back to the first tab if the bound value doesn't match any tab.
	const current = $derived(tabs.some((t) => t.id === activeTab) ? activeTab : tabs[0]?.id ?? '');

	function select(/** @type {string} */ id) {
		const tab = tabs.find((t) => t.id === id);
		if (!tab || tab.disabled || id === activeTab) return;
		activeTab = id;
		onChange?.(id);
	}

	function onKeydown(/** @type {KeyboardEvent} */ e) {
		if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
		const enabled = tabs.filter((t) => !t.disabled);
		const idx = enabled.findIndex((t) => t.id === current);
		if (idx === -1) return;
		const next = e.key === 'ArrowRight'
			? enabled[(idx + 1) % enabled.length]
			: enabled[(idx - 1 + enabled.length) % enabled.length];
		if (next) {
			e.preventDefault();
			select(next.id);
		}
	}
</script>

<div class="tabs">
	<div class="tablist" role="tablist">
		{#each tabs as tab (tab.id)}
			<button
				type="button"
				role="tab"
				class="tab"
				class:active={tab.id === current}
				aria-selected={tab.id === current}
				tabindex={tab.id === current ? 0 : -1}
				disabled={tab.disabled}
				onclick={() => select(tab.id)}
				onkeydown={onKeydown}
			>
				{tab.label}
			</button>
		{/each}
	</div>
	{#if children}
		<div class="tabpanel" role="tabpanel">
			{@render children(current)}
		</div>
	{/if}
</div>

<style>
	.tabs {
		display: flex;
		flex-direction: column;
		min-height: 0;
		height: 100%;
	}

	.tablist {
		display: flex;
		gap: 2px;
		flex: 0 0 auto;
		overflow-x: auto;
		scrollbar-width: thin;
	}

	.tab {
		appearance: none;
		flex: 0 0 auto;
		display: inline-flex;
		align-items: center;
		min-height: 32px;
		padding: 0.25rem 0.75rem;
		background: none;
		border: none;
		border-radius: 6px;
		font: inherit;
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.05em;
		white-space: nowrap;
		color: hsl(var(--color-text-primary));
		cursor: pointer;
	}

	.tab:hover:not(:disabled):not(.active) {
		background: hsl(var(--color-interactive-hover));
	}

	.tab.active {
		background: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}

	.tab:disabled {
		color: hsl(var(--color-text-secondary));
		cursor: not-allowed;
	}

	.tab:focus-visible {
		outline: 2px solid hsl(var(--color-border-focus));
		/* Innen: die scrollende Leiste würde einen Ring außen abschneiden. */
		outline-offset: -2px;
	}

	@media (pointer: coarse) {
		.tab { min-height: 44px; }
	}

	.tabpanel {
		flex: 1 1 auto;
		min-height: 0;
		padding-top: 0.5rem;
	}
</style>
