<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { ApiError, IMAGE_LIMITS, kb } from '$lib/api';
	import { renderMarkdown } from '$lib/markdown';
	import Icon, { type IconName } from '../Icon.svelte';

	// A lightweight Markdown editor: toolbar, keyboard shortcuts, live preview,
	// and image upload (button, drag-and-drop or paste).
	let {
		value = $bindable(''),
		articleId,
		onsave
	}: {
		value?: string;
		/** Images are attached to this article. */
		articleId: number;
		/** ⌘S / Ctrl+S. */
		onsave?: () => void;
	} = $props();

	let ta: HTMLTextAreaElement | undefined = $state();
	let fileInput: HTMLInputElement | undefined = $state();
	let mode = $state<'write' | 'split' | 'preview'>('split');
	let dragging = $state(false);
	let uploading = $state(0);
	let error = $state('');

	onMount(() => {
		if (window.innerWidth < 1024) mode = 'write';
	});

	const preview = $derived(mode === 'write' ? '' : renderMarkdown(value));

	// --- Editing helpers ---

	async function setSelection(start: number, end = start) {
		await tick();
		ta?.focus();
		ta?.setSelectionRange(start, end);
	}

	/** Wrap the selection, e.g. **bold**. Uses `placeholder` if nothing is selected. */
	function wrap(before: string, after = before, placeholder = 'text') {
		if (!ta) return;
		const { selectionStart: s, selectionEnd: e } = ta;
		const selected = value.slice(s, e) || placeholder;
		value = value.slice(0, s) + before + selected + after + value.slice(e);
		setSelection(s + before.length, s + before.length + selected.length);
	}

	/** Add a prefix to each selected line, e.g. "- " for a list (toggles off if already there). */
	function prefixLines(prefix: string | ((i: number) => string)) {
		if (!ta) return;
		const { selectionStart: s, selectionEnd: e } = ta;
		const lineStart = value.lastIndexOf('\n', s - 1) + 1;
		const lineEnd = value.indexOf('\n', e) === -1 ? value.length : value.indexOf('\n', e);
		const lines = value.slice(lineStart, lineEnd).split('\n');
		const p = (i: number) => (typeof prefix === 'string' ? prefix : prefix(i));
		const allHave = lines.every((l, i) => l.startsWith(p(i)));
		const next = lines.map((l, i) => (allHave ? l.slice(p(i).length) : p(i) + l)).join('\n');
		value = value.slice(0, lineStart) + next + value.slice(lineEnd);
		setSelection(lineStart, lineStart + next.length);
	}

	/** Insert a block on its own lines at the cursor. */
	function insertBlock(text: string, selectFrom = 0, selectTo = 0) {
		if (!ta) return;
		const s = ta.selectionStart;
		const before = value.slice(0, s);
		const lead = before === '' || before.endsWith('\n\n') ? '' : before.endsWith('\n') ? '\n' : '\n\n';
		value = before + lead + text + '\n\n' + value.slice(ta.selectionEnd);
		const at = s + lead.length;
		setSelection(at + selectFrom, at + (selectTo || selectFrom));
	}

	function link() {
		if (!ta) return;
		const { selectionStart: s, selectionEnd: e } = ta;
		const label = value.slice(s, e) || 'link text';
		value = value.slice(0, s) + `[${label}](https://)` + value.slice(e);
		const urlStart = s + label.length + 3;
		setSelection(urlStart, urlStart + 8);
	}

	type Tool = { icon: IconName; label: string; keys?: string; run: () => void } | 'sep';
	const tools: Tool[] = [
		{ icon: 'bold', label: 'Heading', run: () => prefixLines('## ') },
		'sep',
		{ icon: 'bold', label: 'Bold', keys: '⌘B', run: () => wrap('**') },
		{ icon: 'italic', label: 'Italic', keys: '⌘I', run: () => wrap('_') },
		{ icon: 'strike', label: 'Strikethrough', run: () => wrap('~~') },
		{ icon: 'code', label: 'Inline code', run: () => wrap('`', '`', 'code') },
		{ icon: 'link', label: 'Link', keys: '⌘K', run: link },
		'sep',
		{ icon: 'listUl', label: 'Bulleted list', run: () => prefixLines('- ') },
		{ icon: 'listOl', label: 'Numbered list', run: () => prefixLines((i) => `${i + 1}. `) },
		{ icon: 'check', label: 'Checklist', run: () => prefixLines('- [ ] ') },
		{ icon: 'quote', label: 'Quote', run: () => prefixLines('> ') },
		'sep',
		{ icon: 'codeBlock', label: 'Code block', run: () => insertBlock('```\ncode\n```', 4, 8) },
		{ icon: 'table', label: 'Table', run: () => insertBlock('| Column | Column |\n| --- | --- |\n| Cell | Cell |', 2, 8) },
		{ icon: 'divider', label: 'Divider', run: () => insertBlock('---') },
		{ icon: 'image', label: 'Image', run: () => fileInput?.click() }
	];

	function onKeydown(e: KeyboardEvent) {
		if (!(e.metaKey || e.ctrlKey)) return;
		const k = e.key.toLowerCase();
		if (k === 'b') wrap('**');
		else if (k === 'i') wrap('_');
		else if (k === 'k') link();
		else if (k === 's') onsave?.();
		else return;
		e.preventDefault();
	}

	// --- Images ---

	async function upload(files: Iterable<File>) {
		error = '';
		for (const file of files) {
			if (!(IMAGE_LIMITS.types as readonly string[]).includes(file.type)) {
				error = `${file.name || 'That file'} isn't a PNG, JPEG, GIF or WebP image.`;
				continue;
			}
			if (file.size > IMAGE_LIMITS.maxBytes) {
				error = `${file.name} is larger than 10 MB.`;
				continue;
			}
			// Drop a placeholder in where the cursor is, then swap it for the real image.
			const alt = (file.name || 'image').replace(/\.[a-z0-9]+$/i, '').replace(/[\[\]]/g, '');
			const placeholder = `![Uploading ${alt}…]()`;
			const at = ta?.selectionStart ?? value.length;
			const block = `${at > 0 && !value.slice(0, at).endsWith('\n') ? '\n' : ''}${placeholder}\n`;
			value = value.slice(0, at) + block + value.slice(at);
			uploading++;
			try {
				const { url } = await kb.uploadImage(articleId, file);
				value = value.replace(placeholder, `![${alt}](${url})`);
			} catch (err) {
				value = value.replace(placeholder + '\n', '').replace(placeholder, '');
				error = err instanceof ApiError && err.status < 500 ? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.' : "Couldn't upload that image.";
			} finally {
				uploading--;
			}
		}
	}

	function onPaste(e: ClipboardEvent) {
		const images = [...(e.clipboardData?.files ?? [])].filter((f) => f.type.startsWith('image/'));
		if (images.length === 0) return;
		e.preventDefault();
		upload(images.map((f, i) => new File([f], `pasted-image-${i + 1}.${f.type.split('/')[1]}`, { type: f.type })));
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		if (e.dataTransfer?.files.length) upload(e.dataTransfer.files);
	}

	const modeBtn = (active: boolean) =>
		['rounded-md px-2.5 py-1 text-xs font-medium transition', active ? 'bg-ink-850 text-ink-100' : 'text-ink-500 hover:text-ink-200'];
</script>

<div class="overflow-hidden rounded-xl border border-ink-800 bg-ink-900/60 focus-within:border-ink-700">
	<!-- Toolbar -->
	<div class="flex flex-wrap items-center gap-0.5 border-b border-ink-800 px-2 py-1.5">
		{#each tools as tool, i (i)}
			{#if tool === 'sep'}
				<span class="mx-1 h-5 w-px bg-ink-800" aria-hidden="true"></span>
			{:else}
				<button
					type="button"
					onclick={tool.run}
					title={tool.keys ? `${tool.label} (${tool.keys})` : tool.label}
					aria-label={tool.label}
					class="flex h-8 min-w-8 items-center justify-center rounded-md px-1.5 text-ink-400 transition hover:bg-ink-850 hover:text-ink-100"
				>
					{#if tool.label === 'Heading'}
						<span class="text-[13px] font-bold">H</span>
					{:else}
						<Icon name={tool.icon} class="size-4" />
					{/if}
				</button>
			{/if}
		{/each}
		<div class="ml-auto flex items-center gap-0.5 rounded-lg border border-ink-800 p-0.5" role="tablist" aria-label="Editor view">
			<button type="button" role="tab" aria-selected={mode === 'write'} onclick={() => (mode = 'write')} class={modeBtn(mode === 'write')}>Write</button>
			<button type="button" role="tab" aria-selected={mode === 'split'} onclick={() => (mode = 'split')} class={[...modeBtn(mode === 'split'), 'hidden lg:block']}>Split</button>
			<button type="button" role="tab" aria-selected={mode === 'preview'} onclick={() => (mode = 'preview')} class={modeBtn(mode === 'preview')}>Preview</button>
		</div>
	</div>

	<div class={['grid', mode === 'split' && 'lg:grid-cols-2']}>
		{#if mode !== 'preview'}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class={['relative', mode === 'split' && 'lg:border-r lg:border-ink-800']}
				ondragenter={(e) => {
					e.preventDefault();
					dragging = true;
				}}
				ondragover={(e) => e.preventDefault()}
				ondragleave={(e) => {
					if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) dragging = false;
				}}
				ondrop={onDrop}
			>
				<label for="kb-body" class="sr-only">Article body (Markdown)</label>
				<textarea
					id="kb-body"
					bind:this={ta}
					bind:value
					onkeydown={onKeydown}
					onpaste={onPaste}
					spellcheck="true"
					placeholder={'Start writing…\n\nUse the toolbar for formatting, or Markdown: ## Heading, **bold**, - list.\nDrag, paste or upload images.'}
					class="block h-[62vh] min-h-80 w-full resize-y bg-transparent px-5 py-4 font-mono text-[13.5px] leading-relaxed text-ink-100 outline-none placeholder:text-ink-500"
				></textarea>
				{#if dragging}
					<div class="pointer-events-none absolute inset-2 flex items-center justify-center rounded-lg border-2 border-dashed border-primary-400 bg-primary-400/10 text-sm font-medium text-primary-200">
						Drop images to add them
					</div>
				{/if}
			</div>
		{/if}
		{#if mode !== 'write'}
			<div class="h-[62vh] min-h-80 overflow-y-auto px-6 py-5">
				{#if value.trim()}
					<div class="kb-prose">{@html preview}</div>
				{:else}
					<p class="text-sm text-ink-500">Nothing to preview yet.</p>
				{/if}
			</div>
		{/if}
	</div>

	<div class="flex items-center justify-between gap-3 border-t border-ink-800 px-4 py-2 text-xs text-ink-500">
		<span>
			{#if uploading}
				Uploading {uploading} {uploading === 1 ? 'image' : 'images'}…
			{:else if error}
				<span class="text-danger-300" role="alert">{error}</span>
			{:else}
				Markdown supported · drag, paste or upload images (10 MB max)
			{/if}
		</span>
		<span class="tabular-nums">{value.length.toLocaleString()} characters</span>
	</div>
	<input
		bind:this={fileInput}
		type="file"
		accept={IMAGE_LIMITS.types.join(',')}
		multiple
		class="sr-only"
		tabindex="-1"
		aria-hidden="true"
		onchange={(e) => {
			upload(e.currentTarget.files ?? []);
			e.currentTarget.value = '';
		}}
	/>
</div>
