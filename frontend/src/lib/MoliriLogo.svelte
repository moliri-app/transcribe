<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Moliris Zeichen, wie auf www.moliri.app (static/favicon.svg), in der Textfarbe. Die Größe gibt
     der Aufrufer über die Klasse; ohne Klasse so groß wie die Schrift. -->
<script>
	/** @type {{ class?: string }} */
	let { class: klasse = '' } = $props();
</script>

<svg class="moliri-logo {klasse}" viewBox="0 0 351 351" aria-hidden="true" focusable="false">
	<path
		fill="currentColor"
		fill-rule="evenodd"
		d="M219.3,7.6C220.1,7.6 220.9,7.9 221.4,8.5L341.9,129.1C342.7,129.9 343,131 342.7,132L298.6,296.6C298.3,297.6 297.5,298.4 296.5,298.7L131.9,342.8C131.6,342.9 131.4,342.9 131.1,342.9C130.3,342.9 129.6,342.6 129,342L8.5,221.5C7.7,220.7 7.4,219.6 7.7,218.6L51.8,53.9C52.1,52.9 52.9,52.1 53.9,51.8L218.5,7.7C218.8,7.6 219,7.6 219.3,7.6ZM201.4,77.5L103.7,103.7L77.5,201.4L149,272.9L246.7,246.7L272.9,149L201.4,77.5Z"
	/>
</svg>

<style>
	.moliri-logo {
		width: 1em;
		height: 1em;
		flex: none;
	}
</style>
