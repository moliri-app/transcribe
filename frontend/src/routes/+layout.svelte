<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<script>
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '@fontsource-variable/atkinson-hyperlegible-next/wght-italic.css';
	import '@fontsource-variable/atkinson-hyperlegible-mono';
	import '../app.scss';
	import { onMount } from 'svelte';
	import AppBar from '$lib/AppBar.svelte';
	import Einrichtung from '$lib/Einrichtung.svelte';
	import Zugriff from '$lib/Zugriff.svelte';

	let { children } = $props();

	// Lebenszeichen an die EXE wie bei den alten Seiten (web/setup.js): ohne sie beendet sie sich,
	// sobald das Fenster zu ist (server.WindowGone). Es sagt auch, ob das Fenster zu sehen ist: wenn
	// nicht, meldet die EXE eine Anfrage mit einer Benachrichtigung (server.WindowInSight).
	onMount(() => {
		const ping = () =>
			fetch(document.visibilityState === 'hidden' ? '/api/ping?hidden' : '/api/ping', { method: 'POST', keepalive: true }).catch(() => {});
		ping();
		const t = setInterval(ping, 5000);
		const bye = () => navigator.sendBeacon('/api/bye');
		addEventListener('pagehide', bye);
		document.addEventListener('visibilitychange', ping);
		return () => {
			clearInterval(t);
			removeEventListener('pagehide', bye);
			document.removeEventListener('visibilitychange', ping);
		};
	});
</script>

<AppBar />
{@render children()}
<Einrichtung />
<Zugriff />
