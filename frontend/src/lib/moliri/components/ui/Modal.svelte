<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Modal.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script module lang="ts">
  // Stack of open modals so Escape + the focus trap only act on the top-most
  // one when modals stack.
  const openModals: symbol[] = [];
</script>

<script lang="ts">
  import Portal from 'svelte-portal';
   import { lockscroll } from '@svelte-put/lockscroll';
   import type { Snippet } from 'svelte';
   import * as m from "$lib/moliri/texte.js";
   import Pager from './Pager.svelte';

  interface Props {
    title?: string;
    isOpen?: boolean;
    size?: 'small' | 'medium' | 'large' | 'xlarge' | 'fullscreen';
    showCloseButton?: boolean;
    header?: boolean | 'auto';
    closeOnBackdropClick?: boolean;
    noPadding?: boolean;
    full?: boolean;
    onClose?: () => void;
    children?: Snippet;
    actions?: Snippet;
    /**
     * Schließen würde Eingaben verwerfen: dann schließt ein Klick neben den Dialog nicht, und
     * ✕ oder Escape fragen vorher. Die Seite vergleicht dafür den Entwurf mit dem Stand beim
     * Öffnen (admin-seitensprache.md §6).
     */
    ungespeichert?: boolean;
    /**
     * Blättern durch die (gefilterte) Liste hinter dem Dialog, ohne ihn zu schließen:
     * „‹ 3 / 22 ›" im Kopf, auch per Alt+↑ / Alt+↓. `position` 0 = der offene Eintrag steht
     * nicht (mehr) in der Liste.
     */
    navigation?: {
      position: number;
      gesamt: number;
      zurueck?: (() => void) | null;
      weiter?: (() => void) | null;
    } | null;
  }

  let {
    title = '',
    isOpen = true,
    size = 'medium',
    showCloseButton = true,
    header = 'auto',
    closeOnBackdropClick = showCloseButton,
    noPadding = false,
    full = false,
    onClose,
    children,
    actions = undefined,
    ungespeichert = false,
    navigation = null
  }: Props = $props();
  // Determine if header should be shown
  let showHeader = $derived(header === true || (header === 'auto' && (title || showCloseButton)));

  // Determine if we need a floating close button (when header is hidden but close button is needed)
  let showFloatingCloseButton = $derived(header === false && showCloseButton);

  /** Jeder Weg hinaus (✕, Escape) läuft hier durch — mit Rückfrage, wenn Eingaben verloren gingen. */
  function schliessen() {
    if (!onClose) return;
    if (ungespeichert && !confirm(m.detail_verwerfen_frage())) return;
    onClose();
  }

  function handleBackdropClick(event: MouseEvent) {
    // Mit ungespeicherten Eingaben schließt ein Klick daneben gar nicht: er passiert zu leicht.
    if (event.target === event.currentTarget && closeOnBackdropClick && !ungespeichert && onClose) {
      onClose();
    }
  }

  function handleCloseClick() {
    schliessen();
  }

  // ---------------------------------------------------------------------------
  // Focus management: aria-modal promises a trap, so deliver one. On open the
  // content container takes focus (so Escape works immediately and SRs enter the
  // dialog); Tab cycles inside; on close focus returns to the opener.
  // ---------------------------------------------------------------------------
  const modalId = Symbol();
  let contentEl: HTMLElement | undefined = $state();
  let restoreFocusEl: HTMLElement | null = null;

  $effect(() => {
    if (!isOpen) return;
    openModals.push(modalId);
    restoreFocusEl = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    queueMicrotask(() => contentEl?.focus());
    return () => {
      const idx = openModals.indexOf(modalId);
      if (idx !== -1) openModals.splice(idx, 1);
      restoreFocusEl?.focus?.();
      restoreFocusEl = null;
    };
  });

  function isTopModal(): boolean {
    return openModals[openModals.length - 1] === modalId;
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (!isOpen || !isTopModal()) return;
    if (event.key === 'Escape') {
      // Window-level so it works before focus ever enters the dialog.
      if (onClose) {
        event.stopPropagation();
        schliessen();
      }
      return;
    }
    // Alt+↑ / Alt+↓ blättert — ohne Alt gehören die Pfeile den Eingabefeldern.
    if (navigation && event.altKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
      const ziel = event.key === 'ArrowUp' ? navigation.zurueck : navigation.weiter;
      if (ziel) {
        event.preventDefault();
        ziel();
      }
      return;
    }
    if (event.key === 'Tab') {
      trapTab(event);
    }
  }

  function trapTab(event: KeyboardEvent) {
    if (!contentEl) return;
    const focusables = contentEl.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement;
    const inside = active instanceof HTMLElement && contentEl.contains(active);
    if (event.shiftKey) {
      if (!inside || active === first) {
        event.preventDefault();
        last.focus();
      }
    } else if (!inside || active === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<svelte:body use:lockscroll={isOpen} />

 <Portal>
{#if isOpen}
  <!-- `toc-exclude`: der Dialog hängt am <body> (Portal) und liegt damit im Suchbereich
       von svelte-toc. Weder der Modal-Titel (h2) noch Überschriften im Inhalt gehören in
       die Gliederung der Seite dahinter. -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
    class="modal-overlay toc-exclude"
    onclick={handleBackdropClick}
    role="dialog"
    aria-modal="true"
    aria-labelledby={title && showHeader ? "modal-title" : undefined}
    aria-label={title && !showHeader ? title : undefined}
  ><div class="modal-content {size ? `size-${size}` : ''}" class:has-floating-close={showFloatingCloseButton} class:full bind:this={contentEl} tabindex="-1">
      <!-- Floating Close Button (when header is disabled but close is needed) -->
      {#if showFloatingCloseButton}
        <button 
          class="modal-floating-close-button" 
          onclick={handleCloseClick}
          aria-label={m.close_modal()}
        >
          <IconMaterialSymbolsCloseRounded />
        </button>
      {/if}

      <!-- Header -->
      {#if showHeader}
        <div class="modal-header" class:no-title={!title}>
          {#if title}
            <h2 id="modal-title" class="modal-title">{title}</h2>
          {/if}

          {#if navigation && navigation.gesamt > 1} <!-- naming:check-ignore (Modal-API `navigation`) -->
            <Pager {...navigation} achse="liste" />
          {/if}

          {#if showCloseButton}
            <button
              type="button"
              class="modal-close-button modal-icon-button"
              onclick={handleCloseClick}
              aria-label={m.close_modal()}
              title={m.close_modal()}
            >
              <IconMaterialSymbolsCloseRounded />
            </button>
          {/if}
        </div>
      {/if}

      <!-- Scrollable Content -->
      {#if children}
        <div class="modal-body" class:no-padding={noPadding}>
          {@render children()}
        </div>
      {/if}

      <!-- Actions Footer -->
      {#if actions}
        <div class="modal-footer">
          {@render actions()}
        </div>
      {/if}
    </div>
  </div>
{/if}
</Portal>
<style lang="scss">
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(47, 48, 49, 0.6); 
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 10000;
    backdrop-filter: blur(10px);
    padding: 1rem;
  }
  .modal-content {
    background: hsl(var(--color-bg-primary));
    border-radius: 12px;
    box-shadow: 0 25px 50px rgba(27, 28, 29, 0.25);
    display: flex;
    flex-direction: column;
    max-height: 90vh;
    overflow: hidden;
    animation: modalEnter 0.3s ease-out;
    position: relative;
    /* focused programmatically on open — the dialog frame needs no visible ring */
    &:focus { outline: none; }
    

    &.size-small {
      width: 90%;
      max-width: 400px;
    }
    
    &.size-medium {
      width: 90%;
      max-width: 600px;
    }
    
    &.size-large {
      width: 90%;
      max-width: 900px;
    }
    
    /* Werkstatt-Details mit Tabellen (Standards: alle Facetten als Spalten) */
    &.size-xlarge {
      width: 95%;
      max-width: 1240px;
    }

    &.size-fullscreen {
      width: 95%;
      max-width: 1600px;
    }

    &.full {
      width: 100% !important;
      height: 100% !important;
      max-width: 100% !important;
      max-height: 100% !important;
      border-radius: 0;
    }

    &.has-floating-close {
      padding-top: 3rem;
    }
  }

  @keyframes modalEnter {
    from {
      opacity: 0;
      transform: scale(0.9) translateY(-20px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }
  /* Kompakter Kopf: eine Zeile Titel (1rem / 700), rechts Blätterer und ✕ als Icon-Knöpfe.
     Der frühere Kopf (1.5rem-Titel, 1.5rem × 2rem Polster) nahm in dichten Editoren eine
     Handbreit Platz weg, ohne mehr zu sagen. */
  .modal-header {
    background: hsl(var(--color-bg-primary));
    border-bottom: 1px solid hsl(var(--color-border-primary));
    color: hsl(var(--color-text-primary));
    min-height: 3.75rem;
    box-sizing: border-box;
    padding: 0.6rem 0.75rem 0.6rem 1.25rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
    border-radius: 12px 12px 0 0;

    &.no-title {
      justify-content: flex-end;
    }

    .modal-title {
      flex: 1;
      min-width: 0;
      margin: 0;
      font-size: 1rem;
      font-weight: 700;
      line-height: 1.3;
    }
  }
  .modal-icon-button {
    flex: none;
    background: none;
    border: none;
    border-radius: 2rem;
    width: 2.25rem;
    height: 2.25rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    color: hsl(var(--color-text-primary));
    transition: background-color 150ms ease-out;

    :global(svg) {
      width: 20px;
      height: 20px;
    }
    &:hover:not(:disabled) {
      background: hsl(var(--color-interactive-hover));
    }
    &:disabled {
      opacity: 0.35;
      cursor: default;
    }
    @media (pointer: coarse) {
      width: 2.75rem;
      height: 2.75rem;
    }
  }
  .modal-floating-close-button {
    position: absolute;
    top: 1rem;
    right: 1rem;
    background: rgba(27, 28, 29, 0.8);
    border: none;
    border-radius: 50%;
    width: 2.5rem;
    height: 2.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
    color: #FAFAFA;
    z-index: 10;
    backdrop-filter: blur(10px);
    
    &:hover {
      background: rgba(27, 28, 29, 0.9);
      transform: scale(1.1);
    }
    
    &:active {
      transform: scale(0.95);
    }
    
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    padding: 2rem;
    color: hsl(var(--color-text-primary));
    font-family: var(--font-ui);
    
    &.no-padding {
      padding: 0;
    }
    
 
    &::-webkit-scrollbar {
      width: 8px;
    }
    
    &::-webkit-scrollbar-track {
      background: hsl(var(--color-bg-secondary));
      border-radius: 10px;
    }

    &::-webkit-scrollbar-thumb {
      background: hsl(var(--color-border-secondary));
      border-radius: 10px;

      &:hover {
        background: hsl(var(--color-text-primary));
      }
    }
  }

  /* Der Fuß liegt AUSSERHALB des Scrollbereichs: die Aktionen stehen immer unten. Maße wie der
     Kopf (Höhe, Polster, Fläche, Trennlinie), damit der Dialog oben und unten gleich gefasst ist. */
  .modal-footer {
    flex-shrink: 0;
    min-height: 3.75rem;
    box-sizing: border-box;
    padding: 0.6rem 0.75rem 0.6rem 1.25rem;
    background: hsl(var(--color-bg-primary));
    border-top: 1px solid hsl(var(--color-border-primary));
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.5rem;
    border-radius: 0 0 12px 12px;
  }


  @media (max-width: 768px) {
    .modal-overlay {
      padding: 0.5rem;
    }
    
    .modal-content {
      width: 100%;
      height: 100%;
      max-height: 100vh;
      border-radius: 0;
      
      &.size-small,
      &.size-medium,
      &.size-large,
      &.size-xlarge,
      &.size-fullscreen {
        width: 100%;
        max-width: none;
      }
    }
    .modal-header {
      padding: 0.5rem 0.5rem 0.5rem 1rem;
      border-radius: 0;

      &.no-title {
        justify-content: flex-end;
      }
    }
    .modal-floating-close-button {
      top: 0.75rem;
      right: 0.75rem;
      width: 2rem;
      height: 2rem;
    }
    
    .modal-body {
      padding: 1.5rem;
    }
    
    .modal-footer {
      padding: 0.6rem 1rem;
      border-radius: 0;
      flex-direction: column;
      align-items: stretch;

      :global(button) {
        width: 100%;
      }
    }
  }

</style>
