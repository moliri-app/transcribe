<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/Dropdown.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- Enhanced Dropdown Component using svelte-floating-ui -->
<script module>
  const openDropdowns = new Set();

  export function closeAllDropdowns() {
    openDropdowns.forEach((dropdown) => {
      dropdown.close();
    });
  }
</script>

<script>
  import { onMount } from 'svelte';
  import { onClickOutside } from 'runed';
  import { createFloatingActions } from 'svelte-floating-ui';
  import { offset, flip, shift } from 'svelte-floating-ui/dom';
  import Portal from 'svelte-portal';

  /** @typedef {'top' | 'bottom' | 'left' | 'right' | 'top-start' | 'top-end' | 'bottom-start' | 'bottom-end' | 'left-start' | 'left-end' | 'right-start' | 'right-end'} FloatingPlacement */
  /** @typedef {'click' | 'hover' | 'rightclick' | 'both' | 'controlled' | 'none'} TriggerMode */
  /** @typedef {'left' | 'right' | 'center'} StandardPosition */
  /** @typedef {'outside' | 'behind'} CloseTrigger */
  /** @typedef {'element' | 'mouse'} PositionMode */
  /** @typedef {'default' | 'basic'} DropdownStyling */
  /** @typedef {{ x: number, y: number } | undefined} MousePosition */
  /**
   * @typedef {{
   *   isOpen?: boolean,
   *   trigger: import('svelte').Snippet,
   *   children: import('svelte').Snippet,
   *   tooltip?: import('svelte').Snippet,
   *   position?: StandardPosition,
   *   minWidth?: string,
   *   closeOnClick?: boolean,
   *   closeTrigger?: CloseTrigger,
   *   usePortal?: boolean,
   *   useFloating?: boolean,
   *   placement?: FloatingPlacement,
   *   offsetDistance?: number,
   *   triggerMode?: TriggerMode,
   *   hoverShowDelay?: number,
   *   hoverHideDelay?: number,
   *   positionMode?: PositionMode,
   *   disabled?: boolean,
   *   mousePosition?: MousePosition,
   *   styling?: DropdownStyling,
   *   doCloseOtherDropdowns?: boolean,
   *   plainTrigger?: boolean,
   *   contentRole?: string
   * } & Omit<import('svelte/elements').HTMLAttributes<HTMLDivElement>, 'children'>} DropdownProps
   */


    //   isOpen?: boolean;
    // trigger: Snippet; 
    // children: Snippet;
    // tooltip?: Snippet;
    // position?: 'left' | 'right' | 'center';
    // minWidth?: string;
    // closeOnClick?: boolean;
    // closeTrigger?: 'outside' | 'behind';
    // usePortal?: boolean;
    // useFloating?: boolean;
    // placement?: 'top' | 'bottom' | 'left' | 'right' | 'top-start' | 'top-end' | 'bottom-start' | 'bottom-end' | 'left-start' | 'left-end' | 'right-start' | 'right-end';
    // offsetDistance?: number;
    // triggerMode?: 'click' | 'hover' | 'rightclick' | 'both' | 'controlled' | 'none';
    // hoverShowDelay?: number;
    // hoverHideDelay?: number;
    // positionMode?: 'element' | 'mouse';
    // mousePosition?: { x: number, y: number };
    // styling?: 'default' | 'basic';
    // doCloseOtherDropdowns?: boolean;
  /** @type {DropdownProps} */
  let {
    isOpen = $bindable(),
    trigger,
    children,
    tooltip = undefined,
    position = 'right',
    minWidth = '120px',
    closeOnClick = true,
    closeTrigger = 'outside',
    usePortal = true,
    useFloating = true,
    placement = 'bottom-end',
    offsetDistance = 8,
    triggerMode = 'click',
    hoverShowDelay = 100,
    hoverHideDelay = 150,
    positionMode = 'element',
    disabled = false,
    mousePosition = $bindable(),// $bindable({ x: 0, y: 0 }),
    styling = 'default',
    doCloseOtherDropdowns = true,
    /** The trigger snippet renders its own interactive element (e.g. a native
     *  <button>) — skip the wrapper's role/tabindex/keydown so keyboard users
     *  get ONE working tab stop instead of a nested double one. */
    plainTrigger = false,
    /** ARIA role of the popover content — pass 'dialog' for rich non-menu
     *  content (settings panels etc.); 'menu' remains the default. */
    contentRole = 'menu',
    ...restProps
  } = $props();
  let triggerElement = $state();
  let dropdownElement = $state();
  let hoverTimeout = $state(null);
  let isHoverMode = $state(false); // Track if we're showing tooltip (hover) or menu (click)

  const isControlledTriggerMode = () => triggerMode === 'controlled' || triggerMode === 'none';

  // Create a dropdown instance object for the registry
  const dropdownInstance = {
    close: () => {
      isOpen = false;
    }
  };
  // Hover event handlers
  function handleMouseEnter() {
    if (disabled || isControlledTriggerMode()) {
      return;
    }

    if (triggerMode === 'hover' && !disabled) {
      // Clear any existing timeout
      if (hoverTimeout) {
        clearTimeout(hoverTimeout);
      }
      // Small delay to prevent flickering
      hoverTimeout = setTimeout(() => {
        if (!isOpen) {
          if (doCloseOtherDropdowns) closeOtherDropdowns();
        }
        isHoverMode = true;
        isOpen = true;
      }, hoverShowDelay);
    }
  }

  function handleMouseLeave() {
    if (disabled || isControlledTriggerMode()) {
      return;
    }

    if (triggerMode === 'hover' && !disabled) {
      // Clear the timeout if we're leaving before it triggered
      if (hoverTimeout) {
        clearTimeout(hoverTimeout);
        hoverTimeout = null;
      }
      // Delay before closing to allow moving to dropdown
      hoverTimeout = setTimeout(() => {
        isOpen = false;
      }, hoverHideDelay);
    }
  }
  // Register this dropdown instance on mount
  onMount(() => {
    openDropdowns.add(dropdownInstance);
    return () => openDropdowns.delete(dropdownInstance);
  });

  function closeOtherDropdowns() {
    openDropdowns.forEach((dropdown) => {
      if (dropdown !== dropdownInstance) {
        dropdown.close();
      }
    });
  }

  // Setup click outside detection with iframe support
  const clickOutside = onClickOutside(
    () => dropdownElement,
    () => {
      if (isOpen) {
        isOpen = false;
      }
    },
    {
      immediate: false, // We'll start/stop this manually
      detectIframe: true // This handles iframe clicks!
    }
  );

  // Start/stop click outside detection when dropdown opens/closes
  $effect(() => {
    if (isOpen && closeTrigger === 'outside') {
      clickOutside.start();
    } else {
      clickOutside.stop();
    }
  });

  $effect(() => {
    if (disabled && isOpen) {
      isOpen = false;
    }
  });

  const floatingOptions = $derived(/** @type {import('svelte-floating-ui').ComputeConfig} */ ({
    placement,
    middleware: [
      offset(offsetDistance),
      flip(),
      shift({ padding: 8 })
    ]
  }));

  // Create floating UI actions when useFloating is enabled
  const [floatingRef, floatingContent] = createFloatingActions({
    strategy: 'fixed'
  });

  // Create mouse-based floating actions when needed
  const [mouseFloatingRef, mouseFloatingContent] = createFloatingActions({
    strategy: 'fixed',
    placement: 'bottom-start',
    middleware: [
      offset({ mainAxis: 10, crossAxis: 10 }),
      flip(),
      shift({ padding: 8 })
    ]
  });

  // Virtual element for mouse positioning (only create once when needed)
  let virtualElement = $state(null);

  // Update virtual element position only when mouse position changes and we're in mouse mode
  $effect(() => {
    if (positionMode === 'mouse' && mousePosition && isOpen) {
      // Only update if position actually changed or if virtualElement doesn't exist
      if (!virtualElement || 
          virtualElement.getBoundingClientRect().x !== mousePosition?.x || 
          virtualElement.getBoundingClientRect().y !== mousePosition?.y) {
        
        virtualElement = {
          getBoundingClientRect() {
            return {
              width: 0,
              height: 0,
              top: mousePosition?.y,
              right: mousePosition?.x,
              bottom: mousePosition?.y,
              left: mousePosition?.x,
              x: mousePosition?.x,
              y: mousePosition?.y,
            };
          },
          contextElement: triggerElement
        };
      }
      // Set the virtual element as reference
      mouseFloatingRef(virtualElement);
    }
  });

  function handleMenuClick() {
    if (closeOnClick) {
      isOpen = false;
    }
  }

  /** @param {KeyboardEvent} event */
  function handleKeydown(event) {
    if (disabled) {
      return;
    }

    if (event.key === 'Escape') {
      // Mark Escape as handled: window-level Escape handlers of surfaces
      // below (e.g. the worker drawer) must not close as well.
      event.preventDefault();
      isOpen = false;
      focusTrigger();
    }
  }

  const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  /** Hand focus back to the trigger. With plainTrigger the wrapper div is not
   *  focusable, so focus the interactive element the snippet rendered. */
  function focusTrigger() {
    const target = plainTrigger ? triggerElement?.querySelector(FOCUSABLE) : triggerElement;
    target?.focus?.();
  }

  // Rich dialog-style popovers opened by click (settings panels) take focus so
  // keyboard users land inside, and give it back to the trigger when they close
  // — unless the user moved focus elsewhere (e.g. clicked into another field).
  $effect(() => {
    if (!isOpen || isHoverMode || contentRole !== 'dialog' || triggerMode !== 'click') return;
    const el = dropdownElement;
    if (!el) return;
    /** @type {HTMLElement} */ (el.querySelector(FOCUSABLE) ?? el).focus({ preventScroll: true });
    return () => {
      const active = document.activeElement;
      if (!active || active === document.body || el.contains(active)) focusTrigger();
    };
  });

  /** @param {KeyboardEvent} event */
  function handleTriggerKeydown(event) {
    // A nested interactive element (native button inside the trigger snippet)
    // synthesizes its own click on Enter/Space — acting here too would
    // double-toggle (open + instantly close).
    if (event.target !== event.currentTarget) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault(); // Space must not scroll the page
      toggleDropdown(event);
    }
  }

  /** Escape must close the dropdown even while focus is still on the trigger
   *  (the content-div handler only fires once focus is inside the popover).
   *  @param {KeyboardEvent} event */
  function handleWindowKeydown(event) {
    if (!isOpen || event.key !== 'Escape') return;
    event.preventDefault();
    isOpen = false;
    focusTrigger();
  }

  // Right-click event handler
  /** @param {MouseEvent} event */
  function handleRightClick(event) {
    if (disabled || isControlledTriggerMode()) {
      return;
    }

    if (event.altKey) {
      isOpen = false;
      return;
    }

    if (triggerMode === 'rightclick' || triggerMode === 'both') {
      event.preventDefault(); // Prevent default context menu
      event.stopPropagation();

      if (positionMode === 'mouse') {
        mousePosition = { x: event.clientX, y: event.clientY };
        virtualElement = {
          getBoundingClientRect() {
            return {
              width: 0,
              height: 0,
              top: event.clientY,
              right: event.clientX,
              bottom: event.clientY,
              left: event.clientX,
              x: event.clientX,
              y: event.clientY,
            };
          },
          contextElement: triggerElement
        };
        mouseFloatingRef(virtualElement);
      }

      if (!isOpen) {
        if (doCloseOtherDropdowns) closeOtherDropdowns();
      }
      isHoverMode = false; // Right-click always shows menu content
      isOpen = !isOpen;
    }
  }

  /** @param {any} event */
  function toggleDropdown(event) {
    if (disabled || isControlledTriggerMode()) {
      return;
    }

    // Only handle click events for click or both modes
    if (triggerMode === 'hover' || triggerMode === 'rightclick') {
      return;
    }
    
    event.stopPropagation(); // Prevent event bubbling
    
    if (positionMode === 'mouse') {
      mousePosition = { x: event.clientX, y: event.clientY };
      virtualElement = {
        getBoundingClientRect() {
          return {
            width: 0,
            height: 0,
            top: event.clientY,
            right: event.clientX,
            bottom: event.clientY,
            left: event.clientX,
            x: event.clientX,
            y: event.clientY,
          };
        },
        contextElement: triggerElement
      };
      mouseFloatingRef(virtualElement);
    }
    
    if (!isOpen) {
      // Close other dropdowns before opening this one
      if (doCloseOtherDropdowns) closeOtherDropdowns();
      isHoverMode = false; // Click mode shows menu content
    }
    
    isOpen = !isOpen;
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if useFloating}
  <!-- Floating UI Implementation -->
  <div class="dropdown-container" {...restProps}>
    {#if positionMode === 'element'}
      <!-- Element-based positioning (default) -->
      <!-- Trigger (plainTrigger: the snippet brings its own interactive element,
           so the wrapper must not be a second tab stop) -->
      <div
        class="dropdown-trigger"
        role={plainTrigger ? undefined : 'button'}
        tabindex={plainTrigger ? undefined : 0}
        bind:this={triggerElement}
        use:floatingRef
        onclick={toggleDropdown}
        oncontextmenu={handleRightClick}
        onkeydown={plainTrigger ? undefined : handleTriggerKeydown}
        onmouseenter={handleMouseEnter}
        onmouseleave={handleMouseLeave}
      >
        {@render trigger()}
      </div>

      <!-- Dropdown Content -->
      {#if isOpen}
        {#if closeTrigger === 'behind'}
          <!-- Background overlay for 'behind' closeTrigger -->
          <button
            type="button"
            class="dropdown-bg"
            aria-label="Close dropdown"
            onclick={() => { isOpen = false; }}
          ></button>
        {/if}
        {#if usePortal}
          <Portal>
            <div 
              bind:this={dropdownElement}
              use:floatingContent={floatingOptions}
              role={contentRole}
              tabindex="-1"
              onclick={handleMenuClick}
              onkeydown={handleKeydown}
              onmouseenter={handleMouseEnter}
              onmouseleave={handleMouseLeave}
              class:basic={styling === 'basic'}
              class="dropdown-content floating"
              style="min-width: {minWidth};"
            >
              {@render children()}
            </div>
          </Portal>
        {:else}
          <div 
            bind:this={dropdownElement}
            use:floatingContent={floatingOptions}
            role={contentRole}
            tabindex="-1"
            onclick={handleMenuClick}
            onkeydown={handleKeydown}
            onmouseenter={handleMouseEnter}
            onmouseleave={handleMouseLeave}
            class:basic={styling === 'basic'}
            class="dropdown-content floating"
            style="min-width: {minWidth};"
          >
            {@render children()}
          </div>
        {/if}
      {/if}
    {:else}
      <!-- Mouse-based positioning -->
      <!-- Trigger (still needed for events, but invisible for positioning) -->
      <div 
        role="button" 
        tabindex="0"
        bind:this={triggerElement}
        onclick={toggleDropdown}
        oncontextmenu={handleRightClick}
        onkeydown={handleTriggerKeydown}
        onmouseenter={handleMouseEnter}
        onmouseleave={handleMouseLeave}
      >
        {@render trigger()}
      </div>

      <!-- Mouse-positioned Dropdown Content -->
      {#if isOpen}
        {#if closeTrigger === 'behind'}
          <!-- Background overlay for 'behind' closeTrigger -->
          <button
            type="button"
            class="dropdown-bg"
            aria-label="Close dropdown"
            onclick={() => { isOpen = false; }}
          ></button>
        {/if}
        {#if usePortal}
          <Portal>
            <div 
              bind:this={dropdownElement}
              use:mouseFloatingContent 
              role={contentRole}
              tabindex="-1"
              onclick={handleMenuClick}
              onkeydown={handleKeydown}
              onmouseenter={handleMouseEnter}
              onmouseleave={handleMouseLeave}
              class:basic={styling === 'basic'}
              class="dropdown-content floating mouse-positioned"
              style="min-width: {minWidth};"
            >
              {@render children()}
            </div>
          </Portal>
        {:else}
          <div 
            bind:this={dropdownElement}
            use:mouseFloatingContent 
            role={contentRole}
            tabindex="-1"
            onclick={handleMenuClick}
            onkeydown={handleKeydown}
            onmouseenter={handleMouseEnter}
            onmouseleave={handleMouseLeave}
            class:basic={styling === 'basic'}
            class="dropdown-content floating mouse-positioned"
            style="min-width: {minWidth};"
          >
            {@render children()}
          </div>
        {/if}
      {/if}
    {/if}
  </div>
{:else}
  <!-- Standard CSS-only Implementation -->
  <div class="dropdown-container" {...restProps}>    <!-- Trigger -->
    <div 
      role="button" 
      tabindex="0"
      bind:this={triggerElement}
      onclick={toggleDropdown}
      oncontextmenu={handleRightClick}
      onkeydown={handleTriggerKeydown}
      onmouseenter={handleMouseEnter}
      onmouseleave={handleMouseLeave}
    >
      {@render trigger()}
    </div>

    <!-- Dropdown Content -->
    {#if isOpen}
      {#if closeTrigger === 'behind'}
        <!-- Background overlay for 'behind' closeTrigger -->
        <button
          type="button"
          class="dropdown-bg"
          aria-label="Close dropdown"
          onclick={() => { isOpen = false; }}
        ></button>
      {/if}
      {#if usePortal}
        <Portal>
          <div 
            bind:this={dropdownElement}
            role={contentRole}
            tabindex="-1"
            onclick={handleMenuClick}
            onkeydown={handleKeydown}
            onmouseenter={handleMouseEnter}
            onmouseleave={handleMouseLeave}
            class:basic={styling === 'basic'}
            class="dropdown-content standard {position}"
            style="min-width: {minWidth};"
          >
            {@render children()}
          </div>
        </Portal>
      {:else}        <div 
          bind:this={dropdownElement}
          role={contentRole}
          tabindex="-1"
          onclick={handleMenuClick}
          onkeydown={handleKeydown}
          onmouseenter={handleMouseEnter}
          onmouseleave={handleMouseLeave}
          class:basic={styling === 'basic'}
          class="dropdown-content standard {position}"
          style="min-width: {minWidth};"
        >
          {@render children()}
        </div>
      {/if}
    {/if}
  </div>
{/if}

<style>
  @layer components {

    .dropdown-trigger {
      display: flex;
    }

    .dropdown-container {
      position: relative;
      display: inline-block;
    }
    .dropdown-bg {
      position: fixed;
      inset: 0;
      background: rgba(0,0,0,0.1);
      border: 0;
      padding: 0;
      z-index: 99997;
    }
    .dropdown-content {
      background: hsl(var(--color-bg-primary)); /* Depotpapier */
      border-radius: 0.5rem; /* Pill designed - no sharp corners */
      box-shadow: rgba(0, 0, 0, 0.2) 0px 3px 8px; /* „Schwebend" (DESIGN.md) */
      z-index: 99998;
      overflow: hidden;
      animation: dropdownIn 0.15s ease-out;
    }

    .dropdown-content.basic {
      /* Basic dropdown styles */
      padding: 0.5rem 1rem;
      font-size: 0.875rem; /* 14px */
      color: hsl(0, 0%, 20%); /* Darker text color */
      line-height: 1.5;
      max-height: 300px; /* Limit height for long menus */
      overflow-y: auto; /* Scroll for long content */
      min-width: 200px; /* Minimum width for basic dropdowns */
    }

    .dropdown-content.floating {
      /* Floating UI handles positioning automatically */
      position: fixed;
      z-index: 99998;
    }

    .dropdown-content.floating.mouse-positioned {
      /* Additional styles for mouse positioning */
      pointer-events: auto;
      max-width: 820px;
      min-width: 280px;
    }
    .dropdown-content.standard {
      position: absolute;
      top: 100%;
      margin-top: 0.25rem;
      z-index: 999999;
    }

    .dropdown-content.standard.left {
      left: 0;
    }

    .dropdown-content.standard.right {
      right: 0;
    }

    .dropdown-content.standard.center {
      left: 50%;
      transform: translateX(-50%);
    }
    @keyframes dropdownIn {
      from {
        opacity: 0;
        transform: translateY(-8px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }
  }
</style>
