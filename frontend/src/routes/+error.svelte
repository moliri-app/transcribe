<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Fehlerseite der Oberfläche: eine Adresse, die es nicht gibt, oder ein Fehler beim Laden. -->
<script>
	import { page } from '$app/state';
</script>

<svelte:head><title>Transcribe by Moliri</title></svelte:head>

<main class="sheet">
	{#if page.status === 404}
		<h1>Diese Seite gibt es nicht</h1>
		<p>Unter dieser Adresse ist nichts. <a href="/editor" data-sveltekit-reload>Zum Editor</a></p>
	{:else}
		<h1>Etwas ist schiefgegangen</h1>
		<p>{page.error?.message ?? `Fehler ${page.status}`}</p>
	{/if}
</main>

<style>
	.sheet {
		max-width: 46rem;
		margin: 2rem auto;
		padding: 1.5rem 2rem 2rem;
		border-radius: 4px;
		background: hsl(var(--color-bg-primary));
	}
	h1 {
		font-size: 1.6rem;
		margin-bottom: 0.75rem;
	}
	p {
		margin-bottom: 0.75rem;
	}
	a {
		text-decoration: underline;
		text-underline-offset: 0.15em;
		font-weight: 600;
	}
</style>
