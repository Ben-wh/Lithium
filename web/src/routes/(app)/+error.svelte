<script lang="ts">
	import { page } from '$app/state';
	import Icon from '$lib/components/Icon.svelte';
</script>

<svelte:head>
	<title>{page.status === 404 ? 'Not found' : 'Error'} · Lithium</title>
</svelte:head>

<div class="mx-auto flex max-w-md flex-col items-center px-4 py-24 text-center">
	<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
		<Icon name="search" class="size-5" />
	</span>
	<h1 class="mt-5 text-xl font-semibold tracking-tight">
		{page.status === 404 ? (page.error?.message ?? 'Page not found') : 'Something went wrong'}
	</h1>
	<p class="mt-2 text-sm text-ink-400">
		{page.status === 404
			? "It may have been removed, or you might not have access to it."
			: 'Please try again in a moment.'}
	</p>
	<a href="/" class="mt-6 text-sm font-medium text-primary-300 hover:text-primary-200">Back to dashboard</a>
</div>
