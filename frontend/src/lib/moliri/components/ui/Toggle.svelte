<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Toggle.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
  import Dropdown from '$lib/moliri/components/ui/Dropdown.svelte';

  let {
    checked = $bindable(),
    disabled = false,
    title = '', 
    description = '',
    tooltip = null,
    icon = null,
    onclick = undefined,
    ...restProps
  } = $props();

  function handleClick(/** @type {MouseEvent} */ event) {
    if (disabled) {
      event.preventDefault();
      return;
    }
    // Toggle the checked state
    checked = !checked;
    onclick?.(event);
  }
</script>

<button 
  type="button"
  role="switch"
  aria-checked={!!checked}
  aria-disabled={disabled || undefined}
  class="toggle-item"
  class:disabled
  onclick={handleClick}
  {...restProps}
>
  <div class="toggle-content">
    <div class="toggle-title-row">
      {#if icon}
        <div class="toggle-icon">
          {@render icon()}
        </div>
      {/if}
      <span class="toggle-title">{title}</span>
      {#if tooltip}
        <Dropdown triggerMode="hover">
          {#snippet trigger()}
            <IconMaterialSymbolsInfoOutlineRounded class="info-icon" />
          {/snippet}
          {#snippet children()}
            <div class="tooltip-content">
              <strong>{@html tooltip.title}</strong>
              <p>{@html tooltip.content}</p>
            </div>
          {/snippet}
        </Dropdown>
      {/if}
    </div>
    {#if description}
      <span class="toggle-desc">{description}</span>
    {/if}
  </div>
  <div class="toggle-switch" class:checked>
    <span class="toggle-slider"></span>
  </div>
</button>

<style>
  .toggle-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem;
    background-color: hsl(156, 12%, 92%);
    border-radius: 0.5rem;
    border: none;
    width: 100%;
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: left;
    margin-top: 0.25rem;
  }

  /* Hover war das Schiefergrau (bg-tertiary): darauf fiel die Beschreibung auf etwa 2:1.
     Jetzt bleibt die Fläche, ein Rand zeigt den Hover. */
  .toggle-item:hover {
    box-shadow: inset 0 0 0 1px hsl(var(--color-border-secondary));
  }

  .toggle-item.disabled {
    opacity: 0.5;
    cursor: not-allowed;
    background-color: hsl(156, 12%, 92%) !important;
  }

  .toggle-item.disabled:hover {
    background-color: hsl(156, 12%, 92%) !important;
    box-shadow: none;
  }

  .toggle-item.disabled .toggle-switch {
    opacity: 0.7;
  }

  .toggle-content {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .toggle-title {
    font-weight: 600;
    color: hsl(210, 4%, 11%);
    font-size: 0.875rem;
    font-family: var(--font-ui);
  }

  .toggle-desc {
    font-size: 0.75rem;
    color: hsl(var(--color-text-secondary));
    font-family: var(--font-ui);
  }

  /* Title row with icon and info */
  .toggle-title-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .toggle-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
    margin-right: 0.125rem;
  }

  .toggle-icon :global(.logo-icon) {
    width: auto;
    height: 16px;
  }

  .toggle-title-row :global(.info-icon) {
    width: 16px;
    height: 16px;
    color: hsl(190, 12%, 61%);
    cursor: pointer;
    transition: color 0.2s ease;
    flex-shrink: 0;
  }

  .toggle-title-row :global(.info-icon:hover) {
    color: hsl(181, 57%, 53%);
  }

  /* Improve icon color visibility on hover */
  .toggle-item:hover .toggle-title-row :global(.info-icon) {
    color: hsl(210, 4%, 11%);
  }

  /* Toggle Switch */
  .toggle-switch {
    position: relative;
    width: 44px;
    height: 24px;
    background-color: hsl(190, 12%, 61%);
    border-radius: 12px;
    transition: background-color 0.2s ease;
    flex-shrink: 0;
  }

  .toggle-item:hover .toggle-switch {
    background-color: hsl(210, 4%, 11%);
  }

  .toggle-item.disabled:hover .toggle-switch {
    background-color: hsl(190, 12%, 61%);
  }

  .toggle-switch.checked {
    background-color: hsl(var(--color-green-100)) !important;
  }

  .toggle-slider {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 20px;
    height: 20px;
    background-color: hsl(0, 0%, 98%);
    border-radius: 50%;
    transition: transform 0.2s ease;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }

  .toggle-switch.checked .toggle-slider {
    transform: translateX(20px);
  }

  /* Tooltip content styling */
  :global(.tooltip-content) {
    padding: 0.75rem;
    background-color: hsl(0, 0%, 98%);
    border-radius: 0.5rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    max-width: 280px;
    font-family: var(--font-ui);
  }

  :global(.tooltip-content strong) {
    display: block;
    font-weight: 600;
    color: hsl(210, 4%, 11%);
    margin-bottom: 0.5rem;
    font-size: 0.875rem;
  }

  :global(.tooltip-content p) {
    margin: 0;
    font-size: 0.8rem;
    color: hsl(210, 4%, 11%);
    line-height: 1.4;
    opacity: 0.8;
  }
</style>
