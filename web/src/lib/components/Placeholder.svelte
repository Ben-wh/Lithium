<script lang="ts">
	import Icon, { type IconName } from './Icon.svelte';

	let { title, description, icon }: { title: string; description: string; icon: IconName } = $props();
</script>

<svelte:head>
	<title>{title} · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<h1 class="text-xl font-semibold tracking-tight">{title}</h1>
	<div
		class="mt-6 flex flex-col items-center justify-center rounded-2xl border border-dashed border-ink-800 px-6 py-20 text-center"
	>
		<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
			<Icon name={icon} class="size-5" />
		</span>
		<p class="mt-4 text-sm font-medium text-ink-200">Coming soon</p>
		<p class="mt-1 max-w-sm text-sm text-ink-500">{description}</p>
	</div>
</div>
