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

	let { children } = $props();

	// Lebenszeichen an die EXE wie bei den alten Seiten (web/setup.js): ohne sie beendet sie sich,
	// sobald das Fenster zu ist (server.WindowGone).
	onMount(() => {
		const ping = () => fetch('/api/ping', { method: 'POST', keepalive: true }).catch(() => {});
		ping();
		const t = setInterval(ping, 5000);
		const bye = () => navigator.sendBeacon('/api/bye');
		addEventListener('pagehide', bye);
		return () => {
			clearInterval(t);
			removeEventListener('pagehide', bye);
		};
	});
</script>

<AppBar />
{@render children()}
<Einrichtung />
