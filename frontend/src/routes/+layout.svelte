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
	import { getLocale } from '$lib/moliri/i18n/locale.js';

	let { children } = $props();

	// A heartbeat to the EXE as the old pages send it (web/setup.js): without it, the EXE ends once
	// the window is closed (server.WindowGone). It also says whether the window is in sight: if not,
	// the EXE announces a request with a notification (server.WindowInSight). And it names the
	// interface language, in which the EXE shows its tray menu, its notifications and the dictation
	// window (cmd/transcribe/texts.go).
	onMount(() => {
		const ping = () =>
			fetch(`/api/ping?lang=${getLocale()}${document.visibilityState === 'hidden' ? '&hidden' : ''}`, {
				method: 'POST',
				keepalive: true
			}).catch(() => {});
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
