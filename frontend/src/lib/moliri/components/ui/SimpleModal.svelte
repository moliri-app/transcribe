<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/SimpleModal.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
    import { lockscroll } from '@svelte-put/lockscroll';
    import * as m from "$lib/moliri/texte.js";
    export let open = true;
    export let padding = '1rem';
    export let display = 'block';
    export let full = false;
    export let showCloseButton = true;

    import Portal from "svelte-portal";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher()

    import { scale } from 'svelte/transition';
	import { quintInOut } from 'svelte/easing';

    function close() {
        open=false;
        dispatch("closed");
		}
  
    </script>

<svelte:body use:lockscroll={open} />
  
  {#if open}
  <Portal target="body">
    <div class="modal-overlay"  transition:scale={{ duration: 200, delay: 0, opacity: 0, start: 0.95, easing: quintInOut}}>
        <div class="modal-outer" class:full>
          <div class="modal-inner" class:full style="padding: {padding}; display: {display};">
              
              <slot />

              {#if showCloseButton}
                <button type="button" class="close" aria-label={m.close()} on:click={close}>
                  <IconMaterialSymbolsClose />
                </button>
              {/if}
          </div>
        </div>
        <button type="button" class="bg" aria-label={m.close_modal()} on:click={close}></button>
    </div>
  </Portal>
  {/if}
  
  <style lang="scss">
  
    .close {
      margin-left: auto;
      cursor: pointer;
      display: block;
      position: absolute;
      top:-1rem;
      right:-1rem;
      background-color: rgba(255, 255, 255, 1);
      padding: 0.4rem;
      border-radius: 50%;
      border: none;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      transition: opacity 0.2s ease-in-out;

      &:hover {
        background-color: rgba(25,25,25,1);
        color: #fff;
      }
    }

    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.2); /* Semi-transparent black overlay */
        z-index: 9997; /* Set a high z-index to overlay the content below */
        display: flex;
        justify-content: center;
        align-items: center;
        backdrop-filter: blur(6px);
    }

    .modal-outer {
        background-color: transparent; /* Set the background color for the modal */
        padding-top: 2rem;
        padding-right: 2rem;
        min-width: 40vw;
        min-height: 40vh;
        max-width: 90%; /* Set the maximum width for the modal */
        max-height: 90%; /* Set the maximum height for the modal */
        // overflow-y: none; /* Enable vertical scrolling if the content exceeds the height */
        z-index: 9999;
        position: relative;
        &.full {
          width: 100vw;
          height: 100vh;
          max-width: 100%;
          max-height: 100%;
        }
    }
  
    .modal-inner {
        min-height: 40vh;
        background-color: #fff; /* Set the background color for the modal */
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2); /* Add a shadow for depth */
        border-radius: 8px;
        max-width: 100%; /* Set the maximum width for the modal */
        max-height:80vh; /* Set the maximum height for the modal */
        overflow: auto; /* Enable vertical scrolling if the content exceeds the height */
        &.full {
          max-width: 100%;
          max-height: 100%;
        }
    }

    .bg {
      position: absolute;
      width: 100%;
      height: 100%;
      z-index: 9998;
      inset: 0;
      border: none;
      background: transparent;
      padding: 0;
    }
  
  </style>