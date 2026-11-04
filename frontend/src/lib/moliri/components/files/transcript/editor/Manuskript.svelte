<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Manuskript.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Manuskript.svelte). -->
<!-- Das Manuskript: Absätze mit Sprecher und Zeitmarke in der Randspalte, Regiezeilen, eingeklappte
     Stellen ausgeblendeter Sprecher. Beim Abspielen werden gesprochene Wörter von Grau zu Tinte, das
     aktuelle wird in Signalgrün überstrichen. Ein Klick auf ein Wort zeigt die Wortleiste (Stelle im
     Text), Strg+Klick spielt sofort ab dort. Der Text ist direkt editierbar: eine kurze Tipppause, Enter
     oder ein Klick daneben übernimmt, Esc verwirft. Eine Auswahl, auch über Sprecher und Absätze hinweg, öffnet den
     Bereichsdialog; Zeitanker lassen sich auf ein anderes Wort ziehen. Wer nicht korrigieren darf, liest
     nur (kein Tippen, keine Anker). Treffer der Suche (ed.hits) sind markiert, der gezeigte umrandet. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { tick, untrack } from 'svelte';
	import { anchorBounds, cueName, fmt, fmtT, gapRange, itemAt, langName, textEdits, wordLang, QUIET_EVENTS } from './model.js';
	import { spkColor } from './editor.svelte.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLDivElement} */ let box;
	/** @type {HTMLDivElement | undefined} */ let bar = $state();
	/** Wortspannen nach Eintrag (zugeklappte fehlen) */
	/** @type {HTMLElement[]} */ let spans = [];
	let cur = -1,
		wasPlaying = false;

	const anchorSet = $derived(new Set(ed.anchors));
	// Clips: vor dem ersten Wort steht ihr Titel, über ihren Wörtern eine Klammer in Tinte
	const clipped = $derived.by(() => {
		/** @type {Map<number, import('./model.js').Clip[]>} */ const starts = new Map();
		/** @type {Set<number>} */ const inside = new Set();
		for (const c of ed.ann.clips) {
			let first = -1;
			ed.items.forEach((it, i) => {
				if (it.kind !== 'w') return;
				const m = (it.start + it.end) / 2;
				if (m < c.start || m > c.end) return;
				inside.add(i);
				if (first < 0) first = i;
			});
			if (first >= 0) starts.set(first, [...(starts.get(first) ?? []), c]);
		}
		return { starts, inside };
	});
	const clipStarts = $derived(clipped.starts);
	const inClip = $derived(clipped.inside);
	const mid = (/** @type {{start: number, end: number}} */ w) => (w.start + w.end) / 2;
	const handSpan = $derived((/** @type {import('./model.js').Eintrag} */ it) =>
		ed.ann.spans.some((m) => it.start >= m.start - 0.01 && it.end <= m.end + 0.01)
	);

	function wordText(/** @type {number} */ i) {
		const it = ed.items[i];
		if (!it.fill) return it.w;
		return (ed.items[i - 1]?.fill ? '' : '(') + it.w + (ed.items[i + 1]?.fill ? '' : ')');
	}
	function wordTitle(/** @type {number} */ i) {
		const it = ed.items[i];
		const L = it.kind === 'w' ? wordLang(it, ed.ann) : null;
		return (
			fmt(it.start) +
			(it.p !== undefined ? ` · ${m.te_confidence({ percent: Math.round(it.p * 100) })}` : '') +
			(L && L.lang !== 'de' ? ` · ${langName(L.lang)}` : '') +
			(it.edited ? ` · ${m.te_sign_edited()}` : it.fill ? ` · ${m.te_filled()}` : it.redone ? ` · ${m.te_redone()}` : '')
		);
	}

	// ---- Mitlesen ----
	// Imperativ statt über Svelte: bei langen Aufnahmen sind es Tausende Wörter, und je Bild
	// ändert sich höchstens eine Handvoll.
	function collect() {
		spans = [];
		for (const el of box.querySelectorAll('[data-i]')) spans[+(/** @type {HTMLElement} */ (el).dataset.i ?? -1)] = /** @type {HTMLElement} */ (el);
		cur = -1;
		paint(true);
	}

	function paint(force = false) {
		const t = ed.time,
			playing = ed.playing,
			items = ed.items,
			i = itemAt(items, t);
		if (i === cur && playing === wasPlaying && !force) return;
		const clearNow = (/** @type {HTMLElement | undefined} */ s) => {
			if (!s) return;
			delete s.dataset.now;
			s.removeAttribute('aria-current');
		};
		if (force) for (const s of spans) if (s) (clearNow(s), delete s.dataset.past);
		clearNow(spans[cur]);
		const a = force ? 0 : Math.max(0, Math.min(cur, i)),
			b = force ? spans.length - 1 : Math.max(cur, i);
		for (let k = a; k <= b && k < spans.length; k++) {
			const s = spans[k];
			if (!s) continue;
			if (k <= i) s.dataset.past = '';
			else delete s.dataset.past;
		}
		const s = spans[i];
		if (s && playing && t <= items[i].end + 0.15) {
			// der Marker läuft in der Zeit über das Wort, in der es gesprochen wird
			s.style.setProperty('--dur', Math.max(0.12, (items[i].end - t) / (ed.rate || 1)).toFixed(2) + 's');
			s.dataset.now = '';
			s.setAttribute('aria-current', 'true');
			if (ed.follow && Date.now() > ed.userScroll) keepVisible(s);
		}
		cur = i;
		wasPlaying = playing;
	}

	function keepVisible(/** @type {HTMLElement} */ s) {
		const r0 = box.getBoundingClientRect(),
			r = s.getBoundingClientRect();
		if (r.top < r0.top + 60 || r.bottom > r0.top + r0.height * 0.62) box.scrollBy({ top: r.top - r0.top - r0.height * 0.32, behavior: 'smooth' });
	}

	$effect(() => {
		ed.text; // nach jedem Neuzeichnen des Texts die Wörter neu einsammeln
		ed.items;
		ed.textGen;
		untrack(collect);
	});
	$effect(() => {
		ed.time;
		ed.playing;
		paint();
	});

	// ---- Suche ----
	// Treffer markiert, der gezeigte umrandet und in die Mitte geholt
	$effect(() => {
		const hits = ed.hits,
			now = ed.hitNow;
		ed.textGen;
		untrack(() => {
			box.querySelectorAll('.treffer').forEach((n) => n.classList.remove('treffer', 'treffer-jetzt'));
			for (const i of hits) spans[i]?.classList.add('treffer');
			if (!now) return;
			for (let i = now.from; i <= now.to; i++) spans[i]?.classList.add('treffer-jetzt');
			spans[now.from]?.scrollIntoView({ block: 'center', behavior: 'smooth' });
		});
	});

	// ---- Klicks im Text ----
	function onclick(/** @type {MouseEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		const cm = /** @type {HTMLElement | null} */ (t.closest('.clipmarke'));
		if (cm) {
			const c = ed.ann.clips.find((k) => k.id === cm.dataset.clip);
			if (!c) return;
			ed.wordbar = null;
			ed.showClip(c);
			ed.clipDlg = { kind: 'clip', clip: c, x: e.clientX, y: e.clientY };
			return;
		}
		const cue = t.closest('.cue-text');
		if (cue) return ed.seek(+(/** @type {HTMLElement} */ (cue).dataset.t ?? 0));
		const w = /** @type {HTMLElement | null} */ (t.closest('.text .w')),
			g = /** @type {HTMLElement | null} */ (t.closest('.text .gap'));
		if (!w && !g) return;
		if (e.ctrlKey || e.metaKey) {
			if (w) ed.seek(ed.items[+(w.dataset.i ?? 0)].start);
			else if (g) ed.seek(gapRange(ed.items, +(g.dataset.after ?? 0)).b);
			return;
		}
		// ein einfacher Klick zeigt die Stelle; eine Auswahl (Bereichsdialog) bleibt in Ruhe
		if (e.detail !== 1 || !getSelection()?.isCollapsed || ed.range) return;
		let i = w ? +(w.dataset.i ?? -1) : ed.items.findIndex((it, j) => j > +(g?.dataset.after ?? 0) && it.kind === 'w');
		if (i < 0) return;
		ed.wordbar = { i, own: false };
		ed.caretT = ed.items[i].start;
	}

	// ---- Tippen ----
	// Ein Absatz ist direkt editierbar. Was getippt ist, wird beim Übernehmen wortweise mit dem
	// verglichen, was er zeigte (textEdits); danach baut der Editor den Text frisch auf (textGen).
	/** @type {HTMLElement | null} */ let editP = null;

	const blockOf = (/** @type {HTMLElement} */ p) => ed.text.blocks.find((b) => b.kind === 'absatz' && b.key === p.dataset.b);
	function tokensOf(/** @type {HTMLElement} */ p) {
		const c = /** @type {HTMLElement} */ (p.cloneNode(true));
		c.querySelectorAll('.ev, .anker, .clipmarke').forEach((n) => n.remove());
		return (c.textContent || '').split(/\s+/).filter(Boolean);
	}
	function commitText(p = editP) {
		clearTimeout(pauseTimer);
		editP = null;
		if (!p) return false;
		const b = /** @type {import('./model.js').Absatz | undefined} */ (blockOf(p));
		const idx = b ? b.idx.filter((i) => ed.items[i].kind === 'w') : [];
		const edits = p.isConnected && idx.length ? textEdits(ed.items, idx, idx.map(wordText), tokensOf(p), ed.ann.edits, ed.appliedN) : null;
		if (edits) {
			ed.commitEdits(edits);
			return true;
		}
		// nichts geändert (oder nur Leerzeichen): der Absatz wird wieder, was er war
		ed.editing = false;
		ed.textGen++;
		ed.status(m.te_no_change());
		return false;
	}
	function discard() {
		clearTimeout(pauseTimer);
		editP = null;
		ed.editing = false;
		ed.textGen++;
		ed.status(m.te_input_discarded());
	}
	ed.textEdit = { commit: () => (editP ? commitText() : false), discard };

	function oninput(/** @type {Event} */ e) {
		const p = /** @type {HTMLElement} */ (e.target).closest?.('.text');
		if (!(p instanceof HTMLElement)) return;
		if (editP && editP !== p) commitText();
		editP = p;
		p.classList.add('offen');
		ed.editing = true;
		ed.wordbar = null;
		ed.range = null;
		ed.status(m.te_input_open(), 'offen');
		clearTimeout(pauseTimer);
		pauseTimer = setTimeout(autoCommit, PAUSE);
	}

	// Nach einer kurzen Tipppause übernimmt der Absatz von selbst und wird gespeichert, ohne Enter.
	// Der Text wird dabei neu aufgebaut; die Schreibmarke steht danach wieder an derselben Stelle im
	// Absatz (gezählt in Zeichen ohne Leerraum), man tippt einfach weiter.
	const PAUSE = 1500;
	let pauseTimer = 0,
		composing = false;
	function autoCommit() {
		const p = editP;
		if (!p?.isConnected || document.activeElement !== p) return;
		// mitten in einem Wort (Buchstaben links und rechts der Marke) wird noch nicht übernommen: sonst
		// würde ein halb getipptes Wort mit seinem Nachbarn verschmolzen
		if (composing || midWord(p)) return void (pauseTimer = setTimeout(autoCommit, PAUSE));
		const at = caretInText(p);
		commitText(p);
		if (at) tick().then(() => placeCaretInText(at));
	}
	/** Textknoten eines Absatzes, die zum Text gehören (ohne Anker und Geräuschnotizen) */
	function textNodes(/** @type {HTMLElement} */ p) {
		/** @type {Text[]} */ const out = [];
		const walk = document.createTreeWalker(p, NodeFilter.SHOW_TEXT);
		for (let n = walk.nextNode(); n; n = walk.nextNode()) if (!n.parentElement?.closest('.anker, .ev, .clipmarke')) out.push(/** @type {Text} */ (n));
		return out;
	}
	function midWord(/** @type {HTMLElement} */ p) {
		const sl = getSelection();
		if (!sl?.rangeCount || !sl.isCollapsed) return false;
		const nodes = textNodes(p),
			i = nodes.indexOf(/** @type {Text} */ (sl.anchorNode));
		if (i < 0) return false;
		const s = nodes[i].textContent ?? '',
			k = sl.anchorOffset;
		const before = k > 0 ? s[k - 1] : (nodes[i - 1]?.textContent ?? '').slice(-1);
		const after = k < s.length ? s[k] : (nodes[i + 1]?.textContent ?? '').slice(0, 1);
		return !!before && !!after && /\S/.test(before) && /\S/.test(after);
	}
	function caretInText(/** @type {HTMLElement} */ p) {
		const sl = getSelection();
		if (!sl?.rangeCount || !sl.isCollapsed || !p.contains(sl.anchorNode)) return null;
		const b = blockOf(p);
		const first = b?.kind === 'absatz' ? b.idx.find((i) => ed.items[i].kind === 'w') : undefined;
		if (first === undefined) return null;
		let n = 0,
			space = false;
		for (const t of textNodes(p)) {
			const s = t === sl.anchorNode ? (t.textContent ?? '').slice(0, sl.anchorOffset) : (t.textContent ?? '');
			if (t === sl.anchorNode || t.compareDocumentPosition(/** @type {Node} */ (sl.anchorNode)) & Node.DOCUMENT_POSITION_FOLLOWING) {
				n += s.replace(/\s/g, '').length;
				if (s) space = /\s$/.test(s);
			}
			if (t === sl.anchorNode) break;
		}
		return { t: ed.items[first].start, n, space };
	}
	function placeCaretInText(/** @type {{ t: number, n: number, space: boolean }} */ at) {
		// der Absatz, dessen erstes Wort dem früheren am nächsten liegt
		/** @type {HTMLElement | null} */ let p = null,
			bd = Infinity;
		for (const el of box.querySelectorAll('.text')) {
			const w = /** @type {HTMLElement | null} */ (el.querySelector('.w:not(.ev)'));
			const it = w ? ed.items[+(w.dataset.i ?? -1)] : null;
			const d = it ? Math.abs(it.start - at.t) : Infinity;
			if (d < bd) ((bd = d), (p = /** @type {HTMLElement} */ (el)));
		}
		if (!p) return;
		let left = at.n,
			/** @type {Text | null} */ node = null,
			off = 0;
		const nodes = textNodes(p);
		for (const t of nodes) {
			const s = t.textContent ?? '';
			let k = 0;
			while (k < s.length && left > 0) if (!/\s/.test(s[k++])) left--;
			node = t;
			off = k;
			if (left === 0) break;
		}
		if (!node) return;
		// stand die Marke hinter einem Leerzeichen, dann wieder dahinter
		if (at.space) {
			const i = nodes.indexOf(node);
			const s = node.textContent ?? '';
			if (off < s.length && /\s/.test(s[off])) off++;
			else if (off === s.length && nodes[i + 1] && /^\s/.test(nodes[i + 1].textContent ?? '')) ((node = nodes[i + 1]), (off = 1));
		}
		p.focus({ preventScroll: true });
		const r = document.createRange();
		r.setStart(node, off);
		r.collapse(true);
		const sl = getSelection();
		sl?.removeAllRanges();
		sl?.addRange(r);
	}
	function onkeydown(/** @type {KeyboardEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		if (!t.isContentEditable) return;
		if (e.key === 'Enter') {
			e.preventDefault();
			/** @type {HTMLElement | null} */ (t.closest('.text'))?.blur();
		} else if (e.key === 'Escape' && editP) {
			e.preventDefault();
			e.stopPropagation();
			discard();
		}
	}
	// Verlassen übernimmt; landet der Klick in einem anderen Absatz, steht die Schreibmarke danach wieder dort
	function onfocusout() {
		if (!editP) return;
		const p = editP;
		setTimeout(() => {
			if (editP !== p || p.contains(document.activeElement)) return;
			const c = box.contains(document.activeElement) ? caretInfo() : null;
			if (commitText(p) || c) tick().then(() => restoreCaret(c));
		}, 0);
	}

	// die Schreibmarke als Zeit: so übersteht sie das Neuzeichnen, und der Abspielknopf startet dort
	function caretEl() {
		const sl = getSelection();
		if (!sl?.rangeCount || !sl.isCollapsed) return null;
		let n = sl.anchorNode;
		if (n instanceof Element && n.matches('.text')) n = n.childNodes[Math.max(0, sl.anchorOffset - 1)]; // am Absatzende
		const el = n?.nodeType === 3 ? n.parentElement : /** @type {Element | null} */ (n);
		return /** @type {HTMLElement | null} */ (el?.closest?.('.text .w, .text .gap') ?? null);
	}
	function caretInfo() {
		const el = caretEl();
		if (!el || !box.contains(el)) return null;
		const gap = el.classList.contains('gap'),
			i = +((gap ? el.dataset.after : el.dataset.i) ?? -1);
		return ed.items[i] ? { t: ed.items[i].start, gap, off: getSelection()?.anchorOffset ?? 0 } : null;
	}
	function restoreCaret(/** @type {{ t: number, gap: boolean, off: number } | null} */ c) {
		if (!c) return;
		let best = -1,
			bd = Infinity;
		ed.items.forEach((it, i) => {
			const d = Math.abs(it.start - c.t);
			if (it.kind === 'w' && d < bd && spans[i]) ((bd = d), (best = i));
		});
		const w = spans[best];
		if (!w?.isConnected) return;
		const el = c.gap && w.nextElementSibling?.classList.contains('gap') ? w.nextElementSibling : w,
			tn = el.firstChild;
		if (!tn) return;
		/** @type {HTMLElement | null} */ (w.closest('.text'))?.focus({ preventScroll: true });
		const r = document.createRange();
		r.setStart(tn, Math.min(c.off, tn.textContent?.length ?? 0));
		r.collapse(true);
		const sl = getSelection();
		sl?.removeAllRanges();
		sl?.addRange(r);
	}
	const wordItemAt = (/** @type {HTMLElement | null} */ el) => {
		if (!el) return -1;
		if (el.classList.contains('gap')) {
			const k = ed.items.findIndex((it, j) => j > +(el.dataset.after ?? 0) && it.kind === 'w');
			return k >= 0 ? k : +(el.dataset.after ?? -1);
		}
		return +(el.dataset.i ?? -1);
	};
	function onselectionchange() {
		const el = caretEl();
		if (!el || !box?.contains(el)) return;
		// mit den Pfeiltasten wandert eine offene Wortleiste mit
		if (ed.wordbar && !ed.editing) {
			const i = wordItemAt(el);
			if (i >= 0 && i !== ed.wordbar.i) ed.wordbar = { i, own: false };
		}
		const c = caretInfo();
		if (!c) return;
		ed.caretT = c.gap ? gapRange(ed.items, +(el.dataset.after ?? 0)).b : c.t;
	}

	// ---- Auswahl ----
	// Innerhalb eines Absatzes wählt der Browser aus. Ein Absatz ist ein eigenes Eingabefeld, darüber
	// hinaus kommt die Auswahl nicht: verlässt das Ziehen den Absatz (anderer Sprecher, neuer Absatz,
	// Regiezeile), verfolgt und zeichnet der Editor die Strecke selbst. Stellen sind halbe Zahlen:
	// Wort i ist i, die Lücke danach i + .5.
	/** @type {{ from: number, to: number, across: boolean, x: number, y: number, raf: number } | null} */ let drag = null;
	const posAt = (/** @type {number} */ x, /** @type {number} */ y) => {
		let n = /** @type {HTMLElement | null} */ (document.elementFromPoint(x, y)?.closest('.manuskript .w, .manuskript .gap') ?? null);
		if (!n) {
			const c = document.caretPositionFromPoint?.(x, y),
				node = c ? c.offsetNode : document.caretRangeFromPoint?.(x, y)?.startContainer;
			const el = node?.nodeType === 3 ? node.parentElement : /** @type {Element | null | undefined} */ (node);
			n = /** @type {HTMLElement | null} */ (el?.closest?.('.manuskript .w, .manuskript .gap') ?? null);
		}
		return !n ? null : n.classList.contains('gap') ? +(n.dataset.after ?? 0) + 0.5 : +(n.dataset.i ?? 0);
	};
	const paraOf = (/** @type {number} */ pos) => spans[Math.floor(pos)]?.closest('.text, .cue-text');
	function dragRange() {
		if (!drag) return null;
		const lo = Math.ceil(Math.min(drag.from, drag.to)),
			hi = Math.floor(Math.max(drag.from, drag.to)),
			ws = [];
		for (let k = lo; k <= hi; k++) if (ed.items[k]?.kind === 'w') ws.push(k);
		return ws.length ? { lo, hi, a: ed.items[ws[0]].start, b: ed.items[ws[ws.length - 1]].end } : null;
	}
	function clearDrag() {
		box?.classList.remove('weit');
		box?.querySelectorAll('.gezogen').forEach((n) => n.classList.remove('gezogen'));
	}
	function drawDrag() {
		clearDrag();
		const r = dragRange();
		ed.sel = r && { a: r.a, b: r.b };
		if (!r) return;
		box.classList.add('weit');
		for (let k = r.lo; k <= r.hi; k++) {
			spans[k]?.classList.add('gezogen');
			const g = spans[k]?.nextElementSibling;
			if (k < r.hi && g?.classList.contains('gap')) g.classList.add('gezogen');
		}
	}
	// der Dialog schließt: die gezeichnete Strecke geht mit
	$effect(() => {
		if (!ed.range) untrack(clearDrag);
	});

	function onpointerdown(/** @type {PointerEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		if (t.closest('.anker')) return ed.canEdit ? grabAnchor(e) : undefined;
		if (t.closest('.clipmarke')) return;
		clearDrag();
		drag = null;
		if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey) return;
		if (!t.closest('.text, .cue-text')) return;
		const from = posAt(e.clientX, e.clientY);
		if (from == null) return;
		const d = (drag = { from, to: from, across: false, x: e.clientX, y: e.clientY, raf: 0 });
		const track = () => {
			const to = posAt(d.x, d.y);
			if (to == null) return;
			d.to = to;
			if (!d.across && paraOf(to) && paraOf(to) !== paraOf(d.from)) d.across = true;
			if (d.across) drawDrag();
		};
		const move = (/** @type {PointerEvent} */ ev) => {
			if (!(ev.buttons & 1)) return up(ev);
			d.x = ev.clientX;
			d.y = ev.clientY;
			track();
		};
		// am oberen oder unteren Rand scrollt der Text weiter, wie bei einer Auswahl des Browsers
		const roll = () => {
			if (drag !== d) return;
			const r = box.getBoundingClientRect(),
				dy = d.y < r.top + 40 ? d.y - r.top - 40 : d.y > r.bottom - 40 ? d.y - r.bottom + 40 : 0;
			if (d.across && dy) {
				box.scrollTop += Math.max(-30, Math.min(30, dy / 2));
				track();
			}
			d.raf = requestAnimationFrame(roll);
		};
		// Loslassen (auch außerhalb des Texts): die Strecke bekommt denselben Dialog wie eine Auswahl im Absatz
		const up = (/** @type {PointerEvent} */ ev) => {
			removeEventListener('pointermove', move);
			removeEventListener('pointerup', up);
			if (drag !== d) return;
			cancelAnimationFrame(d.raf);
			const r = d.across && dragRange();
			drag = null;
			if (!d.across) return;
			getSelection()?.removeAllRanges();
			if (r) openRange(r.a, r.b, ev.clientX, ev.clientY);
			else clearDrag();
		};
		addEventListener('pointermove', move);
		addEventListener('pointerup', up);
		roll();
	}
	// Doppelklick auf ein Wort wählt nur das Wort: Chromium nähme unter Windows das Leerzeichen dahinter
	// mit, und was man dann tippt, verschmölze mit dem nächsten Wort
	function ondblclick(/** @type {MouseEvent} */ e) {
		const w = /** @type {HTMLElement | null} */ (/** @type {HTMLElement} */ (e.target).closest?.('.text .w:not(.ev)'));
		const sl = getSelection();
		if (!w || !sl || !w.isContentEditable) return;
		const r = document.createRange();
		r.selectNodeContents(w);
		sl.removeAllRanges();
		sl.addRange(r);
	}
	function onmouseup(/** @type {MouseEvent} */ e) {
		setTimeout(() => {
			const sl = getSelection();
			if (!sl || sl.isCollapsed || e.detail > 1 || e.ctrlKey || ed.range || !box.contains(sl.anchorNode)) return;
			/** @type {number[]} */ const idx = [];
			spans.forEach((s, i) => s && ed.items[i].kind === 'w' && sl.containsNode(s, true) && idx.push(i));
			if (idx.length) openRange(ed.items[idx[0]].start, ed.items[idx[idx.length - 1]].end, e.clientX, e.clientY);
		}, 0);
	}
	function openRange(/** @type {number} */ a, /** @type {number} */ b, /** @type {number} */ x, /** @type {number} */ y) {
		ed.wordbar = null;
		ed.sel = { a, b };
		ed.range = { a, b, x, y };
	}

	// ---- Zeitanker ziehen ----
	// Ein Anker im Text lässt sich vor ein anderes Wort ziehen: dieses Wort bekommt die Zeit des Ankers.
	/** @type {{ left: number, top: number, height: number, t: string } | null} */ let ghost = $state(null);
	function grabAnchor(/** @type {PointerEvent} */ e) {
		if (e.button !== 0) return;
		e.preventDefault();
		ed.textEdit?.commit();
		const an = /** @type {HTMLElement} */ (/** @type {HTMLElement} */ (e.target).closest('.anker'));
		const wi = +(an.dataset.wi ?? -1),
			words = ed.words;
		if (!words[wi]) return;
		const bd = anchorBounds(words, ed.anchors, wi),
			lo = bd.prevW + 1,
			hi = bd.nextW - 1;
		const spanOf = (/** @type {number} */ w2) => spans[ed.items.findIndex((it) => it.kind === 'w' && it.wi === w2)];
		let target = wi;
		an.classList.add('zieht');
		const place = () => {
			const sp = spanOf(target);
			if (!sp) return;
			const r = sp.getBoundingClientRect();
			ghost = { left: r.left - 2, top: r.top, height: r.height, t: fmtT(words[wi].start) };
			ed.hover = { a: words[target].start, b: words[target].end };
		};
		const move = (/** @type {PointerEvent} */ ev) => {
			const el = /** @type {HTMLElement | null} */ (document.elementFromPoint(ev.clientX, ev.clientY)?.closest('.manuskript .w') ?? null);
			const it = el ? ed.items[+(el.dataset.i ?? -1)] : null;
			if (el && it?.kind === 'w' && it.wi !== undefined) {
				const r = el.getBoundingClientRect();
				target = Math.max(lo, Math.min(hi, it.wi + (ev.clientX > r.left + r.width / 2 ? 1 : 0)));
			}
			place();
		};
		const up = () => {
			removeEventListener('pointermove', move);
			removeEventListener('pointerup', up);
			ghost = null;
			an.classList.remove('zieht');
			ed.hover = null;
			if (target !== wi) ed.moveAnchorTo(words[target].start, words[wi].start, words[wi].start);
		};
		place();
		addEventListener('pointermove', move);
		addEventListener('pointerup', up);
	}

	function onmouseover(/** @type {MouseEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		const w = /** @type {HTMLElement | null} */ (t.closest('.w')),
			g = /** @type {HTMLElement | null} */ (t.closest('.gap'));
		let h = null;
		if (w) {
			const it = ed.items[+(w.dataset.i ?? 0)];
			h = { a: it.start, b: it.end };
		} else if (g) h = gapRange(ed.items, +(g.dataset.after ?? 0));
		if (!h) return;
		ed.hover = h;
		const v = ed.view,
			span = v.b - v.a;
		if (h.b < v.a || h.a > v.b) ed.setView(h.a - span * 0.3, h.a + span * 0.7);
	}

	// ein Clip wird gezeigt (Seitenspalte, Zeitleiste): sein erstes Wort in die Mitte des Texts
	$effect(() => {
		const r = ed.revealT;
		if (!r) return;
		untrack(() => {
			const i = ed.items.findIndex((it) => it.kind === 'w' && (it.start + it.end) / 2 >= r.t);
			const s = /** @type {HTMLElement | undefined} */ (box.querySelector(`.clipmarke[data-clip="${ed.activeClip}"]`) ?? spans[i]);
			if (!s?.isConnected) return;
			ed.userScroll = 0;
			s.scrollIntoView({ block: 'center', behavior: 'smooth' });
		});
	});

	// ---- Wortleiste ----
	const wl = $derived(ed.wordbar && ed.items[ed.wordbar.i] ? { ...ed.wordbar, it: ed.items[ed.wordbar.i] } : null);
	const wlInfo = $derived.by(() => {
		if (!wl) return null;
		const it = wl.it,
			at = mid(it);
		/** @type {{ kind: string, label?: string, value?: string, note?: string, color?: string, clip?: import('./model.js').Clip }[]} */
		const parts = [];
		if (it.kind !== 'w') return [{ kind: 'blau', value: it.w.replace(/^\(|\)$/g, '') }];
		if (it.speaker != null) {
			const bySpan = ed.ann.spans.some((s) => s.speaker !== undefined && s.speaker !== '' && s.start <= at && at <= s.end);
			parts.push({ kind: 'spk', value: ed.spkName(it.speaker), note: bySpan ? m.te_by_hand() : '', color: spkColor(it.speaker) });
		}
		const L = wordLang(it, ed.ann);
		parts.push({ kind: L.by === 'mensch' ? 'rot' : '', label: m.transcript_language(), value: langName(L.lang), note: L.by === 'mensch' ? m.te_by_hand() : L.by === 'maschine' ? m.te_recognised() : '' });
		if (it.edited) parts.push({ kind: 'rot', value: m.te_sign_edited() });
		else if (it.fill) parts.push({ kind: '', value: m.te_filled(), note: m.te_by_machine() });
		else if (it.p !== undefined)
			parts.push({ kind: it.p < 0.5 ? 'zweifel' : '', label: m.te_confidence_label(), value: `${Math.round(it.p * 100)} %`, note: it.p < 0.5 ? m.te_unsure() : '' });
		if (it.redone) parts.push({ kind: 'blau', value: m.te_redone() });
		const ev = (ed.final?.events || []).find((/** @type {any} */ e) => e.start <= at && at <= e.end && !QUIET_EVENTS.includes(e.label));
		if (ev) parts.push({ kind: 'blau', value: ev.label === 'Musik' ? m.te_background_music() : cueName(ev.label) });
		if (it.pin) parts.push({ kind: 'rot', value: m.te_anchor(), note: m.te_by_hand() });
		for (const c of ed.ann.clips) if (c.start <= at && at <= c.end) parts.push({ kind: 'clip', value: c.title, clip: c });
		return parts;
	});
	const wlPlaying = $derived(!!wl?.own && ed.playing);
	let wlPos = $state({ left: 0, top: 0, ax: 0, below: false, hidden: true });

	function placeBar() {
		if (!wl || !bar) return;
		const el = spans[wl.i];
		if (!el?.isConnected) return (ed.wordbar = null);
		const r0 = box.getBoundingClientRect(),
			r = el.getBoundingClientRect();
		if (r.bottom < r0.top || r.top > r0.bottom) return (wlPos = { ...wlPos, hidden: true });
		const w = bar.offsetWidth,
			h = bar.offsetHeight,
			cx = r.left + r.width / 2;
		// in den Grenzen des Manuskripts: die Seitenspalte bleibt frei
		const left = Math.max(r0.left + 10, Math.min(cx - w / 2, r0.right - w - 10, innerWidth - w - 10));
		const below = r.top - h - 12 < r0.top;
		wlPos = { left, top: below ? r.bottom + 12 : r.top - h - 12, ax: Math.max(16, Math.min(w - 16, cx - left)), below, hidden: false };
	}
	$effect(() => {
		wl;
		wlInfo;
		wlPlaying;
		ed.text;
		queueMicrotask(placeBar);
	});

	function wlPlay() {
		if (!wl) return;
		if (wl.own && ed.playing) return ed.media?.pause();
		ed.seek(wl.it.start);
		if (ed.wordbar) ed.wordbar.own = true;
	}

	function outside(/** @type {PointerEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		if (ed.wordbar && !t.closest('.wortleiste') && !t.closest('.text .w, .text .gap')) ed.wordbar = null;
	}
</script>

<svelte:window onresize={placeBar} onpointerdown={outside} />
<svelte:document {onselectionchange} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_mouse_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<div
	class="manuskript"
	class:tippt={ed.editing}
	role="region"
	aria-label={m.file_tab_transcript()}
	bind:this={box}
	{onclick}
	{onmouseover}
	{oninput}
	{onkeydown}
	{onfocusout}
	oncompositionstart={() => (composing = true)}
	oncompositionend={() => (composing = false)}
	{onpointerdown}
	{onmouseup}
	{ondblclick}
	onmouseleave={() => (ed.hover = null)}
	onscroll={placeBar}
	onwheel={() => (ed.userScroll = Date.now() + 4000)}
>
	<div class="blatt">
		{#each ed.text.blocks as b (b.key + ':' + ed.textGen)}
			{#if b.kind === 'klappe'}
				<div class="absatz klappe">
					<div class="rand">
						{#each [...b.spk] as k (k)}
							<span class="knm" style:--c={spkColor(k)}>{ed.spkName(k)}</span>
						{/each}
					</div>
					<div>
						<button
							class="klappe-btn"
							aria-expanded={b.open}
							title={b.open ? m.te_fold() : m.te_unfold_hint()}
							onclick={() => ed.toggleFold(b)}
						>
							<IconMaterialSymbolsChevronRightRounded />
							{b.open ? m.te_unfolded() : m.te_folded()} · <span class="zeit">{fmt(b.start)} – {fmt(b.end)}</span> · {m.te_words({ count: b.n })}
						</button>
					</div>
				</div>
			{:else if b.kind === 'cue'}
				<div class="absatz cue" class:aus={b.aus}>
					<div class="rand">
						<button class="tc" aria-label={m.te_play_from({ time: fmt(b.start) })} onclick={() => ed.seek(b.start)}>{fmt(b.start)}</button>
					</div>
					<button class="cue-text" data-t={b.start}>
						<span class="w ev" data-i={b.i}><b>{cueName(b.label)}</b>{fmt(b.start)} – {fmt(b.end)}</span>
					</button>
				</div>
			{:else}
				<div class="absatz" class:weiter={b.weiter} class:aus={b.aus}>
					<div class="rand">
						{#if !b.weiter && b.spk != null}
							{@const k = b.spk}
							{@const lang = ed.ann.speakers[k]?.language}
							<button
								class="name"
								style:--c={spkColor(k)}
								title={m.te_speaker_edit_hint()}
								aria-haspopup="dialog"
								onclick={(e) => (ed.speakerMenu = { k, anchor: /** @type {HTMLElement} */ (e.currentTarget) })}
								>{ed.spkName(k)}{#if lang}<span class="lang">{langName(lang)}</span>{/if}</button
							>
						{/if}
						<button class="tc" aria-label={m.te_play_from({ time: fmt(b.start) })} onclick={() => ed.seek(b.start)}>{fmt(b.start)}</button>
						{#if ed.isVideo && ed.thumbs}
							{@const tm = ed.thumbs.meta}
							{@const n = Math.max(0, Math.min(tm.count - 1, Math.round(b.start / tm.step)))}
							<button
								class="thumb"
								aria-label={m.te_still_at({ time: fmt(b.start) })}
								style:width="{tm.w * 0.9}px"
								style:height="{tm.h * 0.9}px"
								style:background-image="url(&quot;{ed.thumbs.url}&quot;)"
								style:background-size="{tm.w * tm.cols * 0.9}px {tm.h * tm.rows * 0.9}px"
								style:background-position="-{(n % tm.cols) * tm.w * 0.9}px -{Math.floor(n / tm.cols) * tm.h * 0.9}px"
								onclick={() => ed.seek(b.start)}
							></button>
						{/if}
					</div>
					<p class="text" contenteditable={ed.canEdit ? 'plaintext-only' : 'false'} spellcheck="false" data-b={b.key}>
						{#each b.idx as i, k (i)}
							{@const it = ed.items[i]}
							{#each clipStarts.get(i) ?? [] as c (c.id)}<button
									class="clipmarke"
									contenteditable="false"
									data-clip={c.id}
									data-clip-oeffnet
									class:spielt={ed.playingClip === c.id}
									aria-label={m.te_clip_named({ title: c.title })}
									title={m.te_clip_mark_hint({ from: fmt(c.start), to: fmt(c.end) })}
									><IconMaterialSymbolsContentCutRounded /><span>{c.title}</span></button
								>{/each}{#if it.kind === 'w' && it.wi !== undefined && anchorSet.has(it.wi)}<span
									class="anker"
									class:pin={it.pin}
									contenteditable="false"
									data-wi={it.wi}
									title={it.pin ? m.te_anchor_hint_pinned({ time: fmtT(it.start) }) : m.te_anchor_hint({ time: fmtT(it.start) })}
								></span>{/if}<span
									contenteditable={it.kind === 'ev' ? 'false' : undefined}
								class="w"
								class:ev={it.kind === 'ev'}
								class:fill={it.fill}
								class:redone={it.redone}
								class:edited={it.edited}
								class:low={it.kind === 'w' && it.p !== undefined && it.p < 0.5 && !it.edited}
								class:marked={it.kind === 'w' && handSpan(it)}
								class:imclip={inClip.has(i)}
								data-i={i}
								lang={it.kind === 'w' && wordLang(it, ed.ann).lang !== 'de' ? wordLang(it, ed.ann).lang : undefined}
								title={wordTitle(i)}>{wordText(i)}</span
							><span class="gap" class:imclip={inClip.has(i) && inClip.has(b.idx[k + 1])} data-after={i}>{' '}</span>
						{/each}
					</p>
				</div>
			{/if}
		{/each}
	</div>
</div>

{#if wl && wlInfo}
	<div
		class="wortleiste"
		class:unten={wlPos.below}
		role="toolbar"
		aria-label={m.te_wordbar()}
		bind:this={bar}
		style:left="{wlPos.left}px"
		style:top="{wlPos.top}px"
		style:--ax="{wlPos.ax}px"
		style:visibility={wlPos.hidden ? 'hidden' : undefined}
	>
		<button class="wplay" title={wlPlaying ? m.te_pause() : m.te_play_from_word()} onpointerdown={(e) => e.preventDefault()} onclick={wlPlay}>
			{#if wlPlaying}<IconMaterialSymbolsPauseRounded />{:else}<IconMaterialSymbolsPlayArrowRounded />{/if}
			<span>{wlPlaying ? m.te_pause_short() : m.te_from_here()}</span>
		</button>
		<span class="wi"><span class="zeit">{fmtT(wl.it.start)}</span></span>
		{#each wlInfo as p, k (k)}
			{#if p.clip}
				{@const c = p.clip}
				<button
					class="wi wclip"
					data-clip-oeffnet
					title={m.te_clip_bar_hint({ from: fmt(c.start), to: fmt(c.end) })}
					onclick={(e) => {
						ed.wordbar = null;
						ed.showClip(c);
						ed.clipDlg = { kind: 'clip', clip: c, x: e.clientX, y: e.clientY };
					}}><IconMaterialSymbolsContentCutRounded /><b>{c.title}</b></button
				>
			{:else}
			<span class="wi {p.kind}">
				{#if p.color}<span class="pkt" style:--c={p.color}></span>{/if}
				{#if p.label}{p.label}{/if}
				<b>{p.value}</b>
				{#if p.note}{p.note}{/if}
			</span>
			{/if}
		{/each}
	</div>
{/if}

{#if ghost}
	<div class="anker-ghost" style:left="{ghost.left}px" style:top="{ghost.top}px" style:height="{ghost.height}px" aria-hidden="true">
		<span>{ghost.t}</span>
	</div>
{/if}

<style>
	.manuskript {
		overflow-y: auto;
		padding: 0.75rem 2rem 0;
		min-height: 0;
		scrollbar-width: thin;
		scrollbar-color: var(--rule-2) transparent;
	}
	/* Luft unter dem Text, damit die letzten Zeilen auf Lesehöhe scrollen */
	.manuskript::after {
		content: '';
		display: block;
		height: 40vh;
	}
	.blatt {
		max-width: 1040px;
		margin: 0 auto;
		background: var(--paper);
		border-radius: 0.75rem;
		padding: 1rem 2.75rem 3rem;
		box-shadow: 0 1px 2px hsl(var(--color-neutral-100) / 0.05), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.2);
	}

	.absatz {
		display: grid;
		grid-template-columns: 10.5rem minmax(0, 42rem);
		column-gap: 1.75rem;
		padding: 0.375rem 0;
	}
	.absatz + .absatz:not(.weiter):not(.cue) {
		margin-top: 0.75rem;
		padding-top: 0.75rem;
		border-top: 1px solid var(--rule);
	}
	.absatz.weiter {
		margin-top: 0.5rem;
	}
	/* aufgeklappte Stelle eines ausgeblendeten Sprechers: auf dem Tisch statt auf dem Blatt */
	.absatz.aus {
		background: var(--desk);
		border-radius: 0.5rem;
		margin-inline: -0.75rem;
		padding-inline: 0.75rem;
	}

	.rand {
		text-align: right;
		padding-top: 0.4rem;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.125rem;
	}
	.name {
		background: none;
		border: 0;
		padding: 0.125rem 0;
		color: var(--c);
		font: inherit;
		font-weight: 700;
		font-size: 0.875rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		line-height: 1.3;
		text-align: right;
		cursor: pointer;
		border-radius: 4px;
	}
	.name:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	/* die Sprache auf eigener Zeile unter dem Namen: Namen wie „Plattdeutsch“ brechen sonst den Namen um */
	.name .lang {
		display: block;
		font-weight: 500;
		letter-spacing: 0.02em;
		color: var(--ink-3);
		text-transform: none;
	}
	.tc {
		background: none;
		border: 0;
		padding: 0.125rem 0.375rem;
		margin-right: -0.375rem;
		border-radius: 1rem;
		font: inherit;
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: var(--ink-3);
		cursor: pointer;
	}
	.tc:hover {
		color: var(--ink);
		background: var(--desk);
	}
	.thumb {
		margin-top: 0.375rem;
		border: 1px solid var(--rule);
		border-radius: 4px;
		padding: 0;
		cursor: pointer;
		background-color: #000;
		background-repeat: no-repeat;
	}

	/* Lesetext: groß, ruhig, Wörter noch nicht gesprochen in gedämpfter Tinte */
	.text {
		margin: 0;
		font-size: 1.25rem;
		line-height: 1.75;
		color: var(--future);
		text-wrap: pretty;
		caret-color: var(--ink);
		border-radius: 0.375rem;
		outline: 1.5px dashed transparent;
		outline-offset: 0.375rem;
		transition: outline-color 0.2s;
	}
	.text:focus {
		outline-color: transparent;
	}
	/* getippt, noch nicht übernommen: gestrichelt, denn es ist vorläufig; rot, denn es ist die Hand */
	.text:global(.offen) {
		outline-color: hsl(var(--color-red-200) / 0.5);
		color: var(--ink);
	}
	/* über Absätze hinweg gezogen: die Strecke zeichnet der Editor, die Auswahl des Browsers tritt zurück */
	.manuskript:global(.weit) ::selection {
		background: transparent;
	}
	.manuskript :global(.gezogen) {
		background-color: var(--marker);
		color: var(--ink);
	}
	.w {
		cursor: text;
		border-radius: 3px;
		transition: color 0.35s cubic-bezier(0.16, 1, 0.3, 1);
		background-image: linear-gradient(transparent 56%, var(--marker) 56%, var(--marker) 94%, transparent 94%);
		background-repeat: no-repeat;
		background-size: 0% 100%;
	}
	.w:global([data-past]) {
		color: var(--ink);
	}
	/* das gesprochene Wort: Signalgrün läuft über die Dauer des Worts darüber, wie ein Textmarker */
	.w:global([data-now]) {
		color: var(--ink);
		background-size: 100% 100%;
		transition:
			color 0.2s,
			background-size var(--dur, 0.3s) linear;
	}
	.w.low {
		text-decoration: underline dotted var(--doubt);
		text-decoration-thickness: 2px;
		text-underline-offset: 5px;
	}
	.w.fill,
	.w.fill:global([data-past]) {
		font-style: italic;
		color: var(--fill);
	}
	.w.redone {
		text-decoration: underline solid var(--blue);
		text-decoration-thickness: 1.5px;
		text-underline-offset: 5px;
	}
	/* der Rotstift: alles, was ein Mensch korrigiert hat */
	.w.edited {
		color: var(--red) !important;
		text-decoration: underline solid var(--red);
		text-decoration-thickness: 2px;
		text-underline-offset: 5px;
	}
	.w.marked {
		background-color: var(--sel);
	}
	/* Treffer der Suche: Zweifel-Ocker als Fläche, der gezeigte mit Rahmen */
	.w:global(.treffer) {
		background-color: hsl(var(--color-yellow-100) / 0.45);
	}
	.w:global(.treffer-jetzt) {
		outline: 2px solid var(--doubt);
		outline-offset: 1px;
	}
	.w.ev {
		font-size: 0.95rem;
		font-style: italic;
		color: var(--blue);
	}
	.text .w:hover {
		background-color: var(--desk);
	}

	/* Clips: eine Klammer in Tinte über den Wörtern, wie sie eine Redaktion über eine Passage zieht.
	   Das Polster oben verschiebt keine Zeile, es hebt die Klammer nur über die Buchstaben. */
	.imclip {
		padding-top: 0.3em;
		box-shadow: inset 0 2px 0 var(--clip);
	}
	/* vor dem ersten Wort der Titel als kleine Tintenpille; nimmt am Text nicht teil */
	.clipmarke {
		display: inline-flex;
		align-items: center;
		gap: 0.3125rem;
		max-width: 18em;
		height: 1.625rem;
		margin: 0 0.4375rem 0 0.125rem;
		padding: 0 0.75rem 0 0.5rem;
		vertical-align: 0.12em;
		border: 0;
		border-radius: 2rem;
		background: var(--clip);
		color: var(--clip-ink);
		font: inherit;
		font-size: 0.875rem;
		font-weight: 600;
		line-height: 1;
		user-select: none;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.clipmarke span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.clipmarke :global(svg) {
		width: 1rem;
		height: 1rem;
		margin: 0;
		flex: none;
	}
	.clipmarke:hover {
		background: color-mix(in oklab, var(--clip) 80%, var(--paper));
	}
	/* der Clip läuft: die Schere wird Signal */
	.clipmarke.spielt :global(svg) {
		color: var(--signal);
	}

	/* Zeitanker: dasselbe Fähnchen wie in der Zeitleiste, nimmt keinen Platz */
	.anker {
		display: inline-block;
		position: relative;
		width: 0;
		height: 1em;
		vertical-align: -0.12em;
		user-select: none;
	}
	.anker::before {
		content: '';
		position: absolute;
		left: -1px;
		top: -0.34em;
		width: 8px;
		height: 13px;
		background: var(--ink-3);
		opacity: 0.4;
		mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 8 13'%3E%3Cpath d='M0 0h1.6v13H0z'/%3E%3Cpath d='M1.6 0 8 2.7 1.6 5.4z'/%3E%3C/svg%3E")
			no-repeat 0 0 / 8px 13px;
	}
	.anker.pin::before {
		background: var(--red);
		opacity: 0.9;
	}
	.anker:hover::before,
	.anker:global(.zieht)::before {
		background: var(--blue);
		opacity: 1;
	}
	.anker {
		cursor: grab;
	}
	/* der gezogene Anker vor seinem neuen Wort: gestrichelt, bis er losgelassen ist */
	.anker-ghost {
		position: fixed;
		z-index: 41;
		width: 0;
		border-left: 2px dashed var(--blue);
		pointer-events: none;
	}
	.anker-ghost span {
		position: absolute;
		bottom: 100%;
		left: -1px;
		margin-bottom: 0.25rem;
		padding: 0.125rem 0.5rem;
		border-radius: 1rem;
		background: var(--head);
		color: var(--head-ink);
		font-size: 0.8125rem;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	/* Regiezeilen: Musik, Applaus … eingerückt in Regie-Blau */
	.absatz.cue {
		padding: 0.125rem 0;
	}
	.absatz.cue .rand {
		padding-top: 0.25rem;
	}
	.cue-text {
		justify-self: start;
		background: none;
		border: 0;
		padding: 0.25rem 0.5rem;
		margin-left: -0.5rem;
		border-radius: 0.5rem;
		font: inherit;
		font-size: 1rem;
		color: var(--blue);
		font-style: italic;
		letter-spacing: 0.01em;
		cursor: pointer;
		text-align: left;
	}
	.cue-text:hover {
		background: var(--blue-soft);
	}
	.cue-text .w {
		font-size: inherit;
	}
	.cue-text b {
		font-style: normal;
		font-weight: 700;
		font-size: 0.8125rem;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		margin-right: 0.5rem;
	}

	/* Stellen ausgeblendeter Sprecher, je eine Zeile */
	.absatz.klappe {
		padding: 0.125rem 0;
		align-items: center;
	}
	.absatz + .absatz.klappe,
	.absatz.klappe + .absatz:not(.cue) {
		margin-top: 0.625rem;
		padding-top: 0.625rem;
	}
	.klappe .rand {
		padding-top: 0;
		flex-direction: row;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.125rem 0.625rem;
	}
	.knm {
		color: var(--c);
		font-weight: 700;
		font-size: 0.8125rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.klappe-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1rem 0 0.5rem;
		border: 1px dashed var(--rule-2);
		border-radius: 2rem;
		background: var(--desk);
		color: var(--ink-2);
		font: inherit;
		font-size: 0.9375rem;
		cursor: pointer;
	}
	.klappe-btn:hover {
		color: var(--ink);
		border-color: var(--ink-3);
	}
	.klappe-btn :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		transition: transform 0.15s;
	}
	.klappe-btn[aria-expanded='true'] :global(svg) {
		transform: rotate(90deg);
	}
	.zeit {
		font-variant-numeric: tabular-nums;
	}

	/* Wortleiste: eine Tintenpille über dem Wort, Abspielen gleich daneben */
	.wortleiste {
		position: fixed;
		z-index: 40;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 2px;
		padding: 0.3125rem;
		max-width: min(960px, calc(100vw - 20px));
		background: var(--head);
		color: var(--head-ink);
		border-radius: 1.75rem;
		font-size: 0.9375rem;
		box-shadow: 0 10px 28px -8px hsl(var(--color-neutral-100) / 0.45);
	}
	.wortleiste::after {
		content: '';
		position: absolute;
		left: var(--ax, 50%);
		bottom: -6px;
		width: 12px;
		height: 12px;
		margin-left: -6px;
		background: var(--head);
		transform: rotate(45deg);
		border-radius: 2px;
	}
	.wortleiste.unten::after {
		bottom: auto;
		top: -6px;
	}
	.wplay {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		min-height: var(--hit);
		border: 0;
		border-radius: 2rem;
		padding: 0 1.125rem 0 0.625rem;
		background: var(--signal);
		color: var(--head);
		font: inherit;
		font-weight: 700;
		cursor: pointer;
		position: relative;
		z-index: 1;
	}
	.wplay:hover {
		filter: brightness(1.06);
	}
	.wplay :global(svg) {
		width: 1.5rem;
		height: 1.5rem;
		margin: 0;
	}
	.wi {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 0.75rem;
		color: var(--head-ink-2);
		white-space: nowrap;
		position: relative;
		z-index: 1;
	}
	.wi + .wi {
		border-left: 1px solid hsl(var(--color-neutral-0) / 0.14);
	}
	.wi b {
		color: var(--head-ink);
		font-weight: 600;
	}
	.wi .zeit {
		color: var(--head-ink);
	}
	.pkt {
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--c);
		box-shadow: 0 0 0 1.5px hsl(var(--color-neutral-0) / 0.75);
	}
	.wi.zweifel b {
		color: var(--doubt-on-ink);
	}
	.wi.rot b {
		color: var(--red-on-ink);
	}
	.wi.blau b {
		color: var(--blue-on-ink);
	}
	/* ein Clip an dieser Stelle: anklickbar, öffnet ihn */
	.wclip {
		border: 0;
		border-radius: 2rem;
		background: transparent;
		font: inherit;
		max-width: 18em;
		cursor: pointer;
	}
	.wclip b {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.wclip:hover {
		background: hsl(var(--color-neutral-0) / 0.12);
	}
	.wclip :global(svg) {
		width: 1.125rem;
		height: 1.125rem;
		margin: 0;
		flex: none;
	}

	@media (max-width: 1180px) {
		.manuskript::after {
			height: 30vh;
		}
	}
	@media (max-width: 720px) {
		.manuskript {
			padding: 0.875rem 0.625rem 0;
		}
		.blatt {
			padding: 1.125rem 1rem;
		}
		.absatz {
			grid-template-columns: 1fr;
		}
		.rand {
			flex-direction: row;
			align-items: baseline;
			gap: 0.625rem;
			text-align: left;
			padding-top: 0;
		}
		.tc {
			margin-right: 0;
		}
		.thumb {
			display: none;
		}
		.text {
			font-size: 1.1875rem;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.w,
		.w:global([data-now]) {
			transition: none;
		}
	}
</style>
