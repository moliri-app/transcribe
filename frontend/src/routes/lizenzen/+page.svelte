<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Lizenzübersicht: was die App mitbringt, aus scripts/generate-licenses.mjs, dazu NOTICE. -->
<script>
	import data from '$lib/generated/licenses.json';

	/** @typedef {{ name: string, version?: string, url?: string | null, note?: string }} Entry */
	/** @type {{ key: 'frontend' | 'backend' | 'icons' | 'fonts', title: string, text: string }[]} */
	const sections = [
		{ key: 'frontend', title: 'Oberfläche', text: 'Pakete der Oberfläche mit allem, was sie mitziehen, auch Teile, die nur beim Bauen laufen.' },
		{ key: 'backend', title: 'Programm', text: 'Go-Module, die in der EXE stecken.' },
		{ key: 'icons', title: 'Icons', text: 'Icon-Sammlungen; mitgeliefert werden nur die benutzten Icons.' },
		{ key: 'fonts', title: 'Schriften', text: 'Mitgelieferte Schriften.' }
	];

	/** @param {Record<string, Entry[]>} licenses */
	const count = (licenses) => Object.values(licenses).reduce((n, l) => n + l.length, 0);
</script>

<svelte:head><title>Lizenzen · Transcribe by Moliri</title></svelte:head>

<main>
	<header>
		<h1>Lizenzen</h1>
		<p>
			Transcribe by Moliri ist freie Software unter der
			<a href="https://mozilla.org/MPL/2.0/" target="_blank" rel="noreferrer">Mozilla Public License 2.0</a>.
			Sie baut auf der Arbeit anderer auf; hier steht, was sie mitbringt und unter welcher Lizenz.
		</p>
	</header>

	{#each sections as s (s.key)}
		{@const licenses = /** @type {Record<string, Entry[]>} */ (data[s.key].licenses)}
		<section aria-labelledby="h-{s.key}">
			<h2 id="h-{s.key}">{s.title} <span class="count">{count(licenses)}</span></h2>
			<p class="text">{s.text}</p>
			{#each Object.entries(licenses) as [license, entries] (license)}
				<h3>{license}</h3>
				<ul>
					{#each entries as e (e.name)}
						<li>
							{#if e.url}<a href={e.url} target="_blank" rel="noreferrer">{e.name}</a>{:else}{e.name}{/if}
							{#if e.version}<span class="version">{e.version}</span>{/if}
							{#if e.note}<span class="note">{e.note}</span>{/if}
						</li>
					{/each}
				</ul>
			{:else}
				<p class="text">Keine Angaben.</p>
			{/each}
		</section>
	{/each}

	<section aria-labelledby="h-notice">
		<h2 id="h-notice">Hinweise (NOTICE)</h2>
		<p class="text">
			Was die App beim ersten Start herunterlädt (Erkennungsbibliothek, Modelle, FFmpeg), gehört nicht zu ihr und
			steht mit Lizenz und Namensnennung in der Datei NOTICE.
		</p>
		<details>
			<summary>NOTICE lesen</summary>
			<pre>{data.notice}</pre>
		</details>
	</section>
</main>

<style>
	main {
		max-width: 60rem;
		margin: 0 auto;
		padding: 1.5rem 1.25rem 4rem;
	}
	header {
		margin-bottom: 1.25rem;
	}
	h1 {
		font-size: 1.75rem;
		margin-bottom: 0.5rem;
	}
	h2 {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1.25rem;
		margin-bottom: 0.25rem;
	}
	h3 {
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.05em;
		color: hsl(var(--color-text-secondary));
		margin: 1rem 0 0.25rem;
	}
	a {
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}
	section {
		background: hsl(var(--color-bg-primary));
		border-radius: 4px;
		padding: 1.25rem 1.5rem;
		margin-bottom: 1rem;
	}
	.count {
		min-width: 1.75rem;
		height: 1.5rem;
		padding: 0 0.45rem;
		border-radius: 999px;
		background: hsl(var(--color-bg-secondary));
		font-size: 0.85rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-variant-numeric: tabular-nums;
	}
	.text {
		color: hsl(var(--color-text-secondary));
	}
	ul {
		columns: 2 18rem;
		column-gap: 2rem;
	}
	li {
		break-inside: avoid;
		padding: 0.15rem 0;
	}
	.version {
		margin-left: 0.4rem;
		color: hsl(var(--color-text-secondary));
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
	}
	.note {
		display: block;
		color: hsl(var(--color-text-secondary));
		font-size: 0.875rem;
	}
	details {
		margin-top: 0.75rem;
	}
	summary {
		display: inline-flex;
		align-items: center;
		min-height: var(--hit);
		font-weight: 600;
		gap: 0.4rem;
		cursor: pointer;
	}
	/* der Reset nimmt summary den Pfeil */
	summary::before {
		content: '▸';
		transition: rotate 0.15s;
	}
	details[open] summary::before {
		rotate: 90deg;
	}
	pre {
		margin-top: 0.5rem;
		padding: 1rem;
		border-radius: 4px;
		background: hsl(var(--color-bg-secondary));
		font-family: var(--font-mono);
		font-size: 0.875rem;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
