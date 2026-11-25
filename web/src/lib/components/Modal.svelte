<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		open = $bindable(false),
		title,
		description,
		size = 'md',
		closeOnBackdrop = true,
		children
	}: {
		open?: boolean;
		title: string;
		description?: string;
		/** "lg" for forms that need room, like the template editor. */
		size?: 'md' | 'lg';
		/** Turn off for forms where a stray click outside would lose work. */
		closeOnBackdrop?: boolean;
		children: Snippet;
	} = $props();

	let dialog: HTMLDialogElement | undefined = $state();

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		else if (!open && dialog.open) dialog.close();
	});
</script>

<!-- Native <dialog> handles focus trapping and Escape for us. -->
<dialog
	bind:this={dialog}
	onclose={() => (open = false)}
	onclick={(e) => closeOnBackdrop && e.target === dialog && (open = false)}
	class:max-w-md={size === 'md'}
	class:max-w-2xl={size === 'lg'}
	class="m-auto w-[calc(100%-2rem)] rounded-2xl border border-ink-800 bg-ink-900 text-ink-100 shadow-2xl shadow-shade/60 backdrop:bg-black/60 backdrop:backdrop-blur-sm"
>
	{#if open}
		<div class="p-6">
			<div class="mb-5 flex items-start justify-between gap-4">
				<div>
					<h2 class="text-base font-semibold tracking-tight">{title}</h2>
					{#if description}
						<p class="mt-1 text-sm text-ink-400">{description}</p>
					{/if}
				</div>
				<button
					type="button"
					onclick={() => (open = false)}
					aria-label="Close"
					class="-mt-1 -mr-1 flex size-8 shrink-0 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100"
				>
					<Icon name="close" class="size-4" />
				</button>
			</div>
			{@render children()}
		</div>
	{/if}
</dialog>
