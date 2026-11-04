<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/ui/LoadingWrapper.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<script>
    let { loading = false, message = '', size = 32, children } = $props();
</script>

<div class:is-loading={loading} class="loading-state">
    {#if loading}
        <div class="loading-overlay">
        <span class="loading-icon">
            <IconEosIconsLoading size="{size}px" />
        </span>
        {#if message}
            <span class="loading-message">{message}</span>
        {/if}
        </div>
    {/if}
    <span class="content">
    {@render children()}
    </span>
</div>

<style>
    .loading-state {
        position: relative;
    }

    .is-loading {
        cursor: wait;
    }

    .loading-overlay {
        position: absolute;
        inset: 0;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        z-index: 10;
    }

    .loading-icon {
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .content {
        opacity: 1;
        transition: opacity 0.3s ease;
    }

    .is-loading .content{
        opacity: 0.5;
        pointer-events: none;
    }


    .loading-message {
        font-size: 1rem;
        color: #444;
        text-align: center;
        margin-top: 0.5rem;
    }
</style>