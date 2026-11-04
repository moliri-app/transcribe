<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/KeyboardTooltip.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
	import Dropdown from '$lib/moliri/components/ui/Dropdown.svelte';

	let {
		textBefore = '',
		textAfter = '',
		shortcuts = [],
		placement = /** @type {any} */ ('top'),
		offsetDistance = 10,
		disabled = false,
		children
	} = $props();
</script>

{#if disabled}
	<span class="keyboard-tooltip-trigger">
		{@render children?.()}
	</span>
{:else}
	<Dropdown
		triggerMode="hover"
		styling="basic"
		closeOnClick={false}
		usePortal={true}
		placement={placement}
		offsetDistance={offsetDistance}
		minWidth="fit-content"
	>
		{#snippet trigger()}
			<span class="keyboard-tooltip-trigger">
				{@render children?.()}
			</span>
		{/snippet}

		<div class="keyboard-tooltip-content" role="tooltip">
			{#if textBefore}
				<span class="keyboard-tooltip-text">{textBefore}</span>
			{/if}

			{#if shortcuts.length > 0}
				<div class="keyboard-tooltip-shortcuts">
					{#each shortcuts as shortcut, shortcutIndex (`${shortcutIndex}:${Array.isArray(shortcut) ? shortcut.join('+') : shortcut?.keys?.join('+') ?? ''}`)}
						{@const keys = Array.isArray(shortcut) ? shortcut : (shortcut?.keys ?? [])}
						<span class="keyboard-tooltip-keys" aria-label={keys.join(' + ')}>
							{#each keys as key, keyIndex (`${shortcutIndex}:${keyIndex}:${key}`)}
									<kbd>{key}</kbd>
								{/each}
						</span>
					{/each}
				</div>
			{/if}

			{#if textAfter}
				<span class="keyboard-tooltip-text">{textAfter}</span>
			{/if}
		</div>
	</Dropdown>
{/if}

<style lang="scss">
	.keyboard-tooltip-trigger {
		display: inline-flex;
		align-items: center;
	}

	.keyboard-tooltip-content {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.45rem;
	}

	.keyboard-tooltip-text {
		font-size: 0.78rem;
		line-height: 1.3;
		color: inherit;
	}

	.keyboard-tooltip-shortcuts {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.45rem;
	}

	.keyboard-tooltip-keys {
		display: inline-flex;
		flex-wrap: wrap;
		gap: 0.35rem;
	}

	kbd {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1.8rem;
		padding: 0.18rem 0.42rem;
		border-radius: 0.45rem;
		border: 1px solid hsl(0 0% 78%);
		background: hsl(0 0% 100%);
		box-shadow: inset 0 -1px 0 rgba(0, 0, 0, 0.08);
		font-size: 0.72rem;
		font-weight: 700;
		line-height: 1;
		color: hsl(0 0% 18%);
		text-transform: none;
	}
</style>