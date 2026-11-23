<script lang="ts">
	import type { PublicCategory } from '$lib/api';
	import CategoryTag from './CategoryTag.svelte';
	import Dropdown, { type DropdownOption } from './Dropdown.svelte';

	// Category picker: colour, name and description for each category.
	let {
		value = $bindable(''),
		options,
		id,
		allowNone = true,
		invalid = false,
		disabled = false,
		size = 'md',
		onchange
	}: {
		value?: number | '';
		options: PublicCategory[];
		id: string;
		/** Offer a "No category" choice (off when a category is mandatory). */
		allowNone?: boolean;
		invalid?: boolean;
		disabled?: boolean;
		size?: 'sm' | 'md';
		onchange?: (value: number | '') => void;
	} = $props();

	const byId = $derived(new Map(options.map((c) => [c.id, c])));
	const items = $derived<DropdownOption<number | ''>[]>([
		...(allowNone ? [{ value: '' as const, label: 'No category' }] : []),
		...options.map((c) => ({ value: c.id, label: c.name, keywords: c.description }))
	]);
</script>

<Dropdown {id} bind:value options={items} {invalid} {disabled} {size} {onchange} placeholder="Choose a category…">
	{#snippet item(opt)}
		{@const c = opt.value === '' ? null : byId.get(opt.value)}
		{#if c}
			<span class="tag-{c.color} flex size-3 shrink-0 items-center justify-center">
				<span class="tag-dot size-2.5 rounded-full"></span>
			</span>
			<span class="min-w-0">
				<span class="block truncate text-sm font-medium text-ink-100">{c.name}</span>
				{#if c.description}<span class="block text-xs text-ink-500">{c.description}</span>{/if}
			</span>
		{:else}
			<span class="size-3 shrink-0 rounded-full border border-dashed border-ink-500"></span>
			<span class="min-w-0">
				<span class="block text-sm text-ink-200">No category</span>
				<span class="block text-xs text-ink-500">Not sure? Leave it and the team will sort it.</span>
			</span>
		{/if}
	{/snippet}
	{#snippet selected(opt)}
		{@const c = opt.value === '' ? null : byId.get(opt.value)}
		{#if c}
			<CategoryTag category={c} class="max-w-full text-xs" />
		{:else}
			<span class="text-ink-400">No category</span>
		{/if}
	{/snippet}
</Dropdown>
