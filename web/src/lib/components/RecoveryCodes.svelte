<script lang="ts">
	import Icon from './Icon.svelte';

	// Shows freshly generated recovery codes (they're never shown again) with
	// copy and download, and makes the user confirm they've saved them.
	let { codes, email, saved = $bindable(false) }: { codes: string[]; email: string; saved?: boolean } = $props();

	let copied = $state(false);

	const text = $derived(
		`Lithium recovery codes for ${email}\n` +
			`Each code works once, if you can't use your authenticator app.\n\n` +
			codes.join('\n') +
			'\n'
	);

	async function copy() {
		try {
			await navigator.clipboard.writeText(codes.join('\n'));
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			/* clipboard blocked; they can still download */
		}
	}

	function download() {
		const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
		const a = Object.assign(document.createElement('a'), { href: url, download: 'lithium-recovery-codes.txt' });
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<div>
	<ul class="grid grid-cols-2 gap-x-4 gap-y-2 rounded-lg border border-ink-800 bg-ink-950/60 px-4 py-3.5 font-mono text-sm text-ink-100">
		{#each codes as c (c)}
			<li class="text-center tracking-wide select-all">{c}</li>
		{/each}
	</ul>
	<div class="mt-3 flex gap-2">
		<button
			type="button"
			onclick={copy}
			class="inline-flex h-8 items-center gap-1.5 rounded-md border border-ink-700 px-2.5 text-xs font-medium text-ink-200 transition hover:border-ink-600 hover:bg-ink-850"
		>
			<Icon name={copied ? 'check' : 'copy'} class="size-3.5" />
			{copied ? 'Copied' : 'Copy'}
		</button>
		<button
			type="button"
			onclick={download}
			class="inline-flex h-8 items-center gap-1.5 rounded-md border border-ink-700 px-2.5 text-xs font-medium text-ink-200 transition hover:border-ink-600 hover:bg-ink-850"
		>
			<Icon name="download" class="size-3.5" /> Download
		</button>
	</div>
	<label class="mt-4 flex cursor-pointer items-start gap-2.5 text-sm text-ink-300 select-none">
		<span class="relative mt-0.5 flex size-4 shrink-0">
			<input
				type="checkbox"
				bind:checked={saved}
				class="peer size-4 cursor-pointer appearance-none rounded border border-ink-700 bg-ink-950 transition checked:border-primary-400 checked:bg-primary-400 focus-visible:ring-3 focus-visible:ring-primary-400/25 focus-visible:outline-none"
			/>
			<svg class="pointer-events-none absolute inset-0 hidden size-4 text-primary-950 peer-checked:block" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<path d="M4 8.5l2.5 2.5L12 5.5" />
			</svg>
		</span>
		I've saved these codes somewhere safe. They won't be shown again.
	</label>
</div>
