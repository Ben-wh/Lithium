<script lang="ts">
	import { IMAGE_LIMITS } from '$lib/api';
	import { formatBytes } from '$lib/format';
	import Icon from './Icon.svelte';

	// Pick, drop or paste images. Files are only sent when the form submits.
	let { files = $bindable([]), disabled = false }: { files?: File[]; disabled?: boolean } = $props();

	const { maxFiles, maxBytes, types } = IMAGE_LIMITS;
	let rejected = $state<string[]>([]);
	let dragging = $state(false);
	let inputEl: HTMLInputElement | undefined = $state();

	// Preview URLs, released whenever the list changes or the component goes away.
	let previews = $state<{ file: File; url: string }[]>([]);
	$effect(() => {
		const ps = files.map((file) => ({ file, url: URL.createObjectURL(file) }));
		previews = ps;
		return () => ps.forEach((p) => URL.revokeObjectURL(p.url));
	});

	const full = $derived(files.length >= maxFiles);

	function add(incoming: Iterable<File>) {
		const errors: string[] = [];
		const next = [...files];
		for (const f of incoming) {
			if (!(types as readonly string[]).includes(f.type)) {
				errors.push(`${f.name || 'That file'} isn't a PNG, JPEG, GIF or WebP image.`);
			} else if (f.size > maxBytes) {
				errors.push(`${f.name} is larger than ${formatBytes(maxBytes)}.`);
			} else if (next.length >= maxFiles) {
				errors.push(`You can attach up to ${maxFiles} images.`);
				break;
			} else {
				next.push(f);
			}
		}
		files = next;
		rejected = errors;
	}

	function remove(i: number) {
		files = files.filter((_, j) => j !== i);
		rejected = [];
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		if (!disabled && e.dataTransfer) add(e.dataTransfer.files);
	}

	// Paste screenshots straight from the clipboard, anywhere on the page.
	function onPaste(e: ClipboardEvent) {
		if (disabled || !e.clipboardData) return;
		const images = [...e.clipboardData.files].filter((f) => f.type.startsWith('image/'));
		if (images.length === 0) return;
		e.preventDefault();
		// Clipboard images are all called "image.png"; give them distinct names.
		add(images.map((f, i) => new File([f], `pasted-image-${Date.now()}-${i + 1}.${f.type.split('/')[1]}`, { type: f.type })));
	}
</script>

<svelte:window onpaste={onPaste} />

<div>
	<div class="mb-1.5 flex items-baseline justify-between">
		<span id="images-label" class="text-[13px] font-medium text-ink-200">
			Images <span class="font-normal text-ink-500">(optional)</span>
		</span>
		<span class="text-xs text-ink-500 tabular-nums">{files.length} / {maxFiles}</span>
	</div>

	{#if previews.length > 0}
		<ul class="mb-3 grid grid-cols-3 gap-2 sm:grid-cols-5" aria-labelledby="images-label">
			{#each previews as p, i (p.url)}
				<li class="group relative aspect-square overflow-hidden rounded-lg border border-ink-800 bg-ink-950">
					<img src={p.url} alt={p.file.name} class="size-full object-cover" />
					<div class="absolute inset-x-0 bottom-0 bg-black/60 px-2 py-1">
						<p class="truncate text-[11px] text-white/90" title={p.file.name}>{p.file.name}</p>
						<p class="text-[10px] text-white/60">{formatBytes(p.file.size)}</p>
					</div>
					<button
						type="button"
						onclick={() => remove(i)}
						{disabled}
						aria-label="Remove {p.file.name}"
						class="absolute top-1.5 right-1.5 flex size-6 items-center justify-center rounded-full bg-black/60 text-white opacity-0 backdrop-blur-sm transition group-hover:opacity-100 hover:bg-black/80 focus-visible:opacity-100 focus-visible:outline-none max-sm:opacity-100"
					>
						<Icon name="close" class="size-3.5" />
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if !full}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			ondragenter={(e) => {
				e.preventDefault();
				dragging = true;
			}}
			ondragover={(e) => e.preventDefault()}
			ondragleave={(e) => {
				if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) dragging = false;
			}}
			ondrop={onDrop}
			class={[
				'flex flex-col items-center justify-center rounded-lg border border-dashed px-4 py-6 text-center transition',
				dragging ? 'border-primary-400 bg-primary-400/10' : 'border-ink-700 bg-ink-950/40',
				disabled && 'opacity-60'
			]}
		>
			<span class="flex size-9 items-center justify-center rounded-lg bg-ink-850 text-ink-400">
				<Icon name={dragging ? 'upload' : 'image'} class="size-4.5" />
			</span>
			<p class="mt-2.5 text-sm text-ink-200">
				{#if dragging}
					Drop to attach
				{:else}
					Drag images here, paste a screenshot, or
					<button
						type="button"
						onclick={() => inputEl?.click()}
						{disabled}
						class="font-medium text-primary-300 underline-offset-2 hover:underline focus-visible:underline focus-visible:outline-none"
					>
						browse
					</button>
				{/if}
			</p>
			<p class="mt-1 text-xs text-ink-500">
				PNG, JPG, GIF or WebP · up to {formatBytes(maxBytes)} each · {maxFiles - files.length} more allowed
			</p>
			<input
				bind:this={inputEl}
				type="file"
				accept={types.join(',')}
				multiple
				{disabled}
				aria-labelledby="images-label"
				onchange={(e) => {
					add(e.currentTarget.files ?? []);
					e.currentTarget.value = '';
				}}
				class="sr-only"
			/>
		</div>
	{/if}

	{#if rejected.length > 0}
		<ul class="mt-2 space-y-0.5 text-xs text-primary-300" aria-live="polite">
			{#each rejected as msg, i (i)}
				<li>{msg}</li>
			{/each}
		</ul>
	{/if}
</div>
