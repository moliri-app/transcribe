<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Button.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  let {
    type = 'primary',
    variant = 'default',
    size = 'medium',
    group = false,
    nativeType = 'button',
    attached = '',
    disabled = false,
    loading = false,
    active = false,
    block = false,
    align = undefined,
    title,
    href,
    onclick,
    iconLeft,
    iconRight,
    children,
    ...restProps
  }: {
    type?: 'primary' | 'secondary' | 'tertiary' | 'group';
    variant?: 'default' | 'positive' | 'negative' | 'inverse';
    size?: 'small' | 'medium' | 'large';
    group?: boolean;
    nativeType?: 'button' | 'submit' | 'reset';
    attached?: '' | 'left' | 'right' | 'both';
    disabled?: boolean;
    loading?: boolean;
    active?: boolean;
    block?: boolean;
    /** Content alignment. Defaults to centered; use 'start' for full-width
     *  menu items so the icon + label group at the left edge. */
    align?: 'start' | 'center' | 'end' | 'between';
    title?: string;
    href?: string;
    onclick?: (event: MouseEvent) => void;
    iconLeft?: Snippet;
    iconRight?: Snippet;
    children?: Snippet;
  } & Omit<HTMLAttributes<HTMLElement>, 'title' | 'onclick' | 'children'> = $props();

  function handleClick(event: MouseEvent) {
    if (disabled || loading) {
      event.preventDefault();
      return;
    }
    onclick?.(event);
  }
</script>

<div class="moliri-button-container"
  class:block={block}
>
  {#if loading}
    <div class="spinner-container">
        <span class="spinner" aria-hidden="true"></span>
    </div>
  {/if}

  {#if href && !disabled && !loading}
  <a
  type={nativeType}
  class="moliri-button"
  {href}
  class:moliri-button-primary={type === 'primary'}
  class:moliri-button-secondary={type === 'secondary'}
  class:moliri-button-tertiary={type === 'tertiary'}
  class:moliri-button-group={group}
  class:moliri-button-default={variant === 'default'}
  class:moliri-button-positive={variant === 'positive'}
  class:moliri-button-negative={variant === 'negative'}
  class:moliri-button-inverse={variant === 'inverse'}
  class:moliri-button-small={size === 'small'}
  class:moliri-button-medium={size === 'medium'}
  class:moliri-button-large={size === 'large'}
  class:attached={attached !== ''}
  class:attached-left={attached === 'left'}
  class:attached-right={attached === 'right'}
  class:attached-both={attached === 'both'}
  class:active={active}
  class:block={block}
  class:align-start={align === 'start'}
  class:align-center={align === 'center'}
  class:align-end={align === 'end'}
  class:align-between={align === 'between'}
  {title}
  {...restProps}
>

    {#if iconLeft}
      <span class="icon-left">{@render iconLeft()}</span>
    {/if}
    {#if children}
      <span class="main-content">{@render children()}</span>
    {/if}
    {#if iconRight}
      <span class="icon-right">{@render iconRight()}</span>
    {/if}

  </a>
{:else}
<button
  type={nativeType}
  class="moliri-button"
  class:moliri-button-primary={type === 'primary'}
  class:moliri-button-secondary={type === 'secondary'}
  class:moliri-button-tertiary={type === 'tertiary'}
  class:moliri-button-group={group}
  class:moliri-button-default={variant === 'default'}
  class:moliri-button-positive={variant === 'positive'}
  class:moliri-button-negative={variant === 'negative'}
  class:moliri-button-inverse={variant === 'inverse'}
  class:moliri-button-small={size === 'small'}
  class:moliri-button-medium={size === 'medium'}
  class:moliri-button-large={size === 'large'}
  class:attached={attached !== ''}
  class:attached-left={attached === 'left'}
  class:attached-right={attached === 'right'}
  class:attached-both={attached === 'both'}
  class:active={active}
  class:block={block}
  class:align-start={align === 'start'}
  class:align-center={align === 'center'}
  class:align-end={align === 'end'}
  class:align-between={align === 'between'}
  class:loading={loading}
  class:disabled
  onclick={handleClick}
  disabled={disabled || loading}
  aria-disabled={disabled || loading}
  aria-busy={loading}
  {title}
  {...restProps}
>

    {#if iconLeft}
      <span class="icon-left">{@render iconLeft()}</span>
    {/if}
    {#if children}
      <span class="main-content">{@render children()}</span>
    {/if}
    {#if iconRight}
      <span class="icon-right">{@render iconRight()}</span>
    {/if}
 
</button>
{/if}
</div>

<style lang="scss">

  .moliri-button-container {
      position: relative;
      display: inline-block;

      :global(input) {
        background-color: transparent;
        border: none;
        color: inherit;
        padding-top: 0;
        padding-bottom: 0;
      }
  }

  .moliri-button {		
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    border: 4px solid;
    border-radius: 2rem;
    cursor: pointer;
    transition: all 0.2s ease;
    text-decoration: none;
    outline: none;
    gap: 0.5rem;
    // text-transform: uppercase;
    text-align: center;
    width: max-content;
    max-width: 100%;
    line-height: 1.2em;
    letter-spacing: 0.05em;

    --moliri-button-dark: var(--color-neutral-100);
    --moliri-button-light: var(--color-neutral-0);
    --moliri-button-light-hover: var(--color-neutral-10);
    --moliri-button-dark-hover: var(--color-neutral-50);

    &.attached {
        height: 100%;

    }
    &.attached-left {
        border-bottom-right-radius: 0;
        border-top-right-radius: 0;
    }
    &.attached-right {
        border-bottom-left-radius: 0;
        border-top-left-radius: 0;
    }
    &.attached-both {
        border-radius: 0;
    }

    span {
        min-height: 2em;
        display: flex;
        align-items: center;

        &.main-content {
            width: 100%;
            justify-content: center;
        }
    }
  }

  .block {
    width: 100%;
    justify-content: space-between;
  }

  /* Content alignment override — targets the inner .main-content (which is
     width:100%), so it works even for full-width `block` buttons. */
  .moliri-button {
    &.align-start .main-content { justify-content: flex-start; }
    &.align-center .main-content { justify-content: center; }
    &.align-end .main-content { justify-content: flex-end; }
    &.align-between .main-content { justify-content: space-between; }
  }

  /* Klickfläche: mindestens 32px, unter `pointer: coarse` 44px — ohne die sichtbare Größe
     zu ändern. Ein unsichtbares ::after ragt über den Rand hinaus, soweit der Knopf kleiner
     ist (`50% - 16px` ist bei großen Knöpfen positiv, dann greift 0). Weil es nichts am
     Layout ändert, bricht keine dichte Leiste um; zwischen eng stehenden Knöpfen gewinnt
     der später gezeichnete die Überlappung. `min-height` nur unter `coarse` hätte die
     Leisten auf dem iPad um ein Drittel aufgebläht. */
  .moliri-button {
    position: relative;
    --hit: 32px;

    &::after {
      content: '';
      position: absolute;
      inset: min(0px, calc(50% - var(--hit) / 2));
    }
  }
  @media (pointer: coarse) {
    .moliri-button { --hit: 44px; }
  }

  /* Größen */
  .moliri-button-small { padding: 0rem 0.3rem; font-size: 0.72rem; }
  .moliri-button-medium { padding: 0.1rem 0.5rem; font-size: 0.85rem; }
  .moliri-button-large { padding: 0.75rem 1rem; font-size: 1rem; }

  /* loading Spinner */
  .spinner-container {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 1em;
    height: 1em;
  }
  .spinner {
    display: block;
    width: 100%;
    height: 100%;
    border: 2px solid currentColor;
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

.loading {
   opacity: 0.2;
  }
  /* Der Spinner liegt mittig über dem Knopf: die Beschriftung macht ihm Platz
     (Breite bleibt, damit der Knopf nicht springt), sonst überdeckt er den Text. */
  .moliri-button.loading > span {
    visibility: hidden;
  }

/* Default variant (uses moliri-button-dark and moliri-button-light variables) */
  /* Primary type */
  .moliri-button-primary.moliri-button-default {
    background-color: hsl(var(--moliri-button-dark));
    border-color: hsl(var(--moliri-button-dark));
    color: hsl(var(--moliri-button-light));
  }
  /* Hover war Schiefergrau (neutral-50) unter Papier: 2,5:1. Jetzt Tinte, um ein Fünftel
     zum Papier hin gemischt: 10,6:1, und der Hover bleibt sichtbar. */
  .moliri-button-primary.moliri-button-default:hover:not(.disabled) {
    background-color: color-mix(in oklab, hsl(var(--moliri-button-dark)) 82%, hsl(var(--moliri-button-light)));
    border-color: color-mix(in oklab, hsl(var(--moliri-button-dark)) 82%, hsl(var(--moliri-button-light)));
  }
  .moliri-button-primary.moliri-button-inverse {
    background-color: hsl(var(--moliri-button-light));
    border-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-dark));
  }
  .moliri-button-primary.moliri-button-inverse:hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light-hover));
    border-color: hsl(var(--moliri-button-light-hover));
    color: hsl(var(--moliri-button-dark));
  }

  /* Secondary type */
  .moliri-button-secondary.moliri-button-default {
    background-color: transparent;
    border-color: hsl(var(--moliri-button-dark));
    color: hsl(var(--moliri-button-dark));
  }
  .moliri-button-secondary.moliri-button-default:hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-dark));
    color: hsl(var(--moliri-button-light));
  }
  .moliri-button-secondary.moliri-button-inverse {
    background-color: transparent;
    border-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-light));
  }
  .moliri-button-secondary.moliri-button-inverse:hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-dark));
  }

  /* Tertiary type */
  .moliri-button-tertiary.moliri-button-default {
    background-color: transparent;
    border-color: transparent;
    color: hsl(var(--moliri-button-dark));
  }
  .moliri-button-tertiary.moliri-button-default:hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light-hover));
  }
  .moliri-button-tertiary.moliri-button-inverse {
    background-color: transparent;
    border-color: transparent;
    color: hsl(var(--moliri-button-light));
  }
  .moliri-button-tertiary.moliri-button-inverse:hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-dark));
  }

  /* Dunkles Thema. Die Default-Variante malt mit fester Tinte auf Papier; auf dunklem
     Grund war Tertiär damit unsichtbar und Primär verschmolz mit dem Dialogfuß.
     Primär bleibt dunkel und bekommt einen hellen Rand (nicht invertiert, keine
     Akzentfarbe), Sekundär einen grauen Rand, Tertiär nur helle Schrift: die
     Rangfolge bleibt ablesbar. Inverse ist schon für dunkle Flächen gebaut und bleibt.
     Gruppenknöpfe (Umschalter wie Tabelle/Objekt/Stapel, Vergleichsoperatoren) bleiben
     ausgenommen: Ihre Leisten haben eine eigene, auch im dunklen Thema helle Fläche,
     auf der helle Schrift verschwände. */
  :global(.theme-dark) .moliri-button-primary.moliri-button-default:not(.moliri-button-group) {
    background-color: hsl(var(--moliri-button-dark));
    border-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-light));
  }
  :global(.theme-dark) .moliri-button-primary.moliri-button-default:not(.moliri-button-group):hover:not(.disabled) {
    background-color: color-mix(in oklab, hsl(var(--moliri-button-dark)) 82%, hsl(var(--moliri-button-light)));
    border-color: hsl(var(--moliri-button-light));
  }
  :global(.theme-dark) .moliri-button-secondary.moliri-button-default:not(.moliri-button-group) {
    border-color: hsl(var(--moliri-button-dark-hover));
    color: hsl(var(--moliri-button-light));
  }
  :global(.theme-dark) .moliri-button-secondary.moliri-button-default:not(.moliri-button-group):hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light));
    border-color: hsl(var(--moliri-button-light));
    color: hsl(var(--moliri-button-dark));
  }
  :global(.theme-dark) .moliri-button-tertiary.moliri-button-default:not(.moliri-button-group) {
    color: hsl(var(--moliri-button-light));
  }
  /* Durchscheinend statt fester Ton: trägt auf jedem dunklen Grund (Seite, Dialog, Leiste). */
  :global(.theme-dark) .moliri-button-tertiary.moliri-button-default:not(.moliri-button-group):hover:not(.disabled) {
    background-color: hsl(var(--moliri-button-light) / 0.12);
  }

  /* Positive variant (green coloring) */
  /* Primary type */
  .moliri-button-primary.moliri-button-positive {
    background-color: hsl(var(--color-green-100)); /* #98F600 */
    border-color: hsl(var(--color-green-100));
    color: hsl(var(--color-neutral-100)); /* Black text */
  }

  .moliri-button-primary.moliri-button-positive:hover:not(.disabled) {
    background-color: hsl(var(--color-green-50)); /* Lighter on hover */
    border-color: hsl(var(--color-green-50));
    /* opacity: 0.9; */
  }

  /* Secondary type */
  .moliri-button-secondary.moliri-button-positive {
    background-color: transparent;
    border-color: hsl(var(--color-green-100)); /* #98F600 */
    color: hsl(var(--color-green-100));
  }

  .moliri-button-secondary.moliri-button-positive:hover:not(.disabled) {
    background-color: hsl(var(--color-green-100)); /* Fill on hover */
    color: hsl(var(--color-neutral-100)); /* Black text */
  }

  /* Tertiary type */
  .moliri-button-tertiary.moliri-button-positive {
    background-color: transparent;
    border-color: transparent;
    color: hsl(var(--color-green-100)); /* #98F600 */
  }

  .moliri-button-tertiary.moliri-button-positive:hover:not(.disabled) {
    background-color: hsl(var(--color-green-100)); /* Fill on hover */
    color: hsl(var(--color-neutral-100)); /* Black text */
  }

  /* Negative variant (red coloring) */
  /* Primary type */
  /* Rot als Fläche unter Papier und als Schrift in der dunklen Stufe: red-100 hatte mit
     Papier nur 3,9:1 (Schrift auf Weiß 4,1:1), red-200 hat 6,5:1. */
  .moliri-button-primary.moliri-button-negative {
    background-color: hsl(var(--color-red-200));
    border-color: hsl(var(--color-red-200));
    color: hsl(var(--color-neutral-0)); /* White text */
  }

  .moliri-button-primary.moliri-button-negative:hover:not(.disabled) {
    background-color: color-mix(in oklab, hsl(var(--color-red-200)) 85%, hsl(var(--color-neutral-100)));
    border-color: color-mix(in oklab, hsl(var(--color-red-200)) 85%, hsl(var(--color-neutral-100)));
  }

  /* Secondary type */
  .moliri-button-secondary.moliri-button-negative {
    background-color: transparent;
    border-color: hsl(var(--color-red-200));
    color: hsl(var(--color-red-200));
  }

  .moliri-button-secondary.moliri-button-negative:hover:not(.disabled) {
    background-color: hsl(var(--color-red-200)); /* Fill on hover */
    color: hsl(var(--color-neutral-0)); /* White text */
  }

  /* Tertiary type */
  .moliri-button-tertiary.moliri-button-negative {
    background-color: transparent;
    border-color: transparent;
    color: hsl(var(--color-red-200));
  }

  .moliri-button-tertiary.moliri-button-negative:hover:not(.disabled) {
    background-color: hsl(var(--color-red-200)); /* Fill on hover */
    color: hsl(var(--color-neutral-0)); /* White text */
  }

  /* Dunkles Thema: red-200 als Schrift und Rand hat auf Tinte nur 2,5:1, red-50 rund 7:1.
     Gefüllt (Primär, Hover) bleibt es bei red-200 mit weißer Schrift. */
  :global(.theme-dark) .moliri-button-secondary.moliri-button-negative:not(:hover:not(.disabled)) {
    border-color: hsl(var(--color-red-50));
    color: hsl(var(--color-red-50));
  }
  :global(.theme-dark) .moliri-button-tertiary.moliri-button-negative:not(:hover:not(.disabled)) {
    color: hsl(var(--color-red-50));
  }


  .disabled, .moliri-button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .loading {
    cursor: wait !important;
  }

  /* Active state */
  .active { 
    /* transform: translateY(1px); */
    transform: scale(0.95);
     }

      /* Group prop */
  .moliri-button-group {
    border-radius: 8px;
    margin: 0 1px;
  }
</style>
