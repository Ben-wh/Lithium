<script lang="ts" module>
	export type DropdownOption<T> = {
		value: T;
		/** Used for type-ahead, search and screen readers. */
		label: string;
		/** Extra words to match when searching. */
		keywords?: string;
	};
</script>

<script lang="ts" generics="T extends string | number">
	import { tick, type Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	// A styled listbox dropdown. Each option is drawn by the `item` snippet
	// (and the closed button by `selected`, defaulting to `item`), so options
	// can show badges, icons, avatars and descriptions. Keyboard: arrows,
	// Home/End, Enter/Space, Escape, and type-ahead (or a search box).
	let {
		id,
		value = $bindable(),
		options,
		item,
		selected: selectedSnippet,
		placeholder = 'Select…',
		disabled = false,
		invalid = false,
		searchable = false,
		size = 'md',
		menu = 'stretch',
		label,
		onchange
	}: {
		id: string;
		value: T;
		options: DropdownOption<T>[];
		item: Snippet<[DropdownOption<T>]>;
		selected?: Snippet<[DropdownOption<T>]>;
		placeholder?: string;
		disabled?: boolean;
		invalid?: boolean;
		/** Show a search box (worth it for long lists, e.g. people). */
		searchable?: boolean;
		size?: 'sm' | 'md';
		/** "stretch" matches the button's width; "wide" is a wider menu for narrow buttons. */
		menu?: 'stretch' | 'wide';
		/** Accessible name if there's no <label for={id}>. */
		label?: string;
		onchange?: (value: T) => void;
	} = $props();

	let open = $state(false);
	let active = $state(0);
	let query = $state('');
	let rootEl: HTMLElement | undefined = $state();
	let buttonEl: HTMLButtonElement | undefined = $state();
	let listEl: HTMLUListElement | undefined = $state();
	let searchEl: HTMLInputElement | undefined = $state();

	const current = $derived(options.find((o) => o.value === value));
	const shown = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return options;
		return options.filter((o) => `${o.label} ${o.keywords ?? ''}`.toLowerCase().includes(q));
	});

	async function openList() {
		if (disabled) return;
		query = '';
		active = Math.max(0, options.findIndex((o) => o.value === value));
		open = true;
		await tick();
		(searchable ? searchEl : listEl)?.focus();
	}

	function close(refocus = true) {
		open = false;
		if (refocus) buttonEl?.focus();
	}

	function choose(opt: DropdownOption<T> | undefined) {
		if (!opt) return;
		const changed = opt.value !== value;
		value = opt.value;
		close();
		if (changed) onchange?.(opt.value);
	}

	// Keep the highlighted option in view while arrowing through a long list.
	$effect(() => {
		if (open) document.getElementById(`${id}-option-${active}`)?.scrollIntoView({ block: 'nearest' });
	});
	// Searching resets the highlight to the first match.
	$effect(() => {
		query;
		active = 0;
	});

	function onButtonKeydown(e: KeyboardEvent) {
		if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
			e.preventDefault();
			openList();
		}
	}

	let typed = '';
	let typedTimer: ReturnType<typeof setTimeout>;

	function onListKeydown(e: KeyboardEvent) {
		const last = shown.length - 1;
		switch (e.key) {
			case 'ArrowDown':
				active = Math.min(active + 1, last);
				break;
			case 'ArrowUp':
				active = Math.max(active - 1, 0);
				break;
			case 'Home':
				if (searchable) return; // let the search box move its cursor
				active = 0;
				break;
			case 'End':
				if (searchable) return;
				active = last;
				break;
			case 'Enter':
				choose(shown[active]);
				break;
			case ' ':
				if (searchable) return; // typing a space in the search box
				choose(shown[active]);
				break;
			case 'Escape':
				close();
				break;
			case 'Tab':
				close(false);
				return;
			default:
				if (searchable) return;
				// Type-ahead: jump to the first option starting with what's been typed.
				if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
					typed += e.key.toLowerCase();
					clearTimeout(typedTimer);
					typedTimer = setTimeout(() => (typed = ''), 600);
					const i = shown.findIndex((o) => o.label.toLowerCase().startsWith(typed));
					if (i >= 0) active = i;
					break;
				}
				return;
		}
		e.preventDefault();
		e.stopPropagation();
	}

	function onWindowClick(e: MouseEvent) {
		if (open && rootEl && !rootEl.contains(e.target as Node)) close(false);
	}
</script>

<svelte:window onclick={onWindowClick} />

<div class="relative" bind:this={rootEl}>
	<button
		bind:this={buttonEl}
		{id}
		type="button"
		{disabled}
		onclick={() => (open ? close() : openList())}
		onkeydown={onButtonKeydown}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-controls="{id}-listbox"
		aria-label={label}
		class={[
			'field flex cursor-pointer items-center gap-2 pr-9 text-left disabled:cursor-not-allowed disabled:opacity-60',
			size === 'sm' && 'h-9 text-[13px]',
			(open || invalid) && 'border-primary-400/70 ring-3 ring-primary-400/15'
		]}
	>
		<span class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
			{#if current}
				{@render (selectedSnippet ?? item)(current)}
			{:else}
				<span class="truncate text-ink-500">{placeholder}</span>
			{/if}
		</span>
		<Icon
			name="chevron"
			class={['pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-ink-500 transition', open && 'rotate-180'].join(' ')}
		/>
	</button>

	{#if open}
		<div
			class={[
				'absolute top-full z-30 mt-1.5 overflow-hidden rounded-xl border border-ink-800 bg-ink-900 shadow-2xl shadow-shade/40',
				menu === 'wide' ? 'left-0 w-64 max-w-[calc(100vw-2rem)]' : 'inset-x-0'
			]}
		>
			{#if searchable}
				<div class="relative border-b border-ink-800">
					<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
					<input
						bind:this={searchEl}
						bind:value={query}
						onkeydown={onListKeydown}
						role="combobox"
						aria-controls="{id}-listbox"
						aria-expanded="true"
						aria-activedescendant={shown[active] ? `${id}-option-${active}` : undefined}
						aria-label="Search"
						placeholder="Search…"
						class="h-10 w-full bg-transparent pr-3 pl-9 text-sm text-ink-100 outline-none placeholder:text-ink-500"
					/>
				</div>
			{/if}
			<ul
				bind:this={listEl}
				id="{id}-listbox"
				role="listbox"
				tabindex="-1"
				aria-labelledby={label ? undefined : id}
				aria-label={label}
				aria-activedescendant={!searchable && shown[active] ? `${id}-option-${active}` : undefined}
				onkeydown={searchable ? undefined : onListKeydown}
				class="max-h-80 overflow-y-auto p-1.5 outline-none"
			>
				{#each shown as opt, i (opt.value)}
					{@const isSelected = opt.value === value}
					<!-- Keyboard selection is handled on the list / search box (aria-activedescendant). -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id="{id}-option-{i}"
						role="option"
						aria-selected={isSelected}
						onmousedown={(e) => e.preventDefault()}
						onclick={() => choose(opt)}
						onmousemove={() => (active = i)}
						class={['flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2', i === active && 'bg-ink-850']}
					>
						<span class="flex min-w-0 flex-1 items-center gap-3">{@render item(opt)}</span>
						{#if isSelected}
							<Icon name="check" class="size-4 shrink-0 text-primary-400" />
						{/if}
					</li>
				{:else}
					<li class="px-3 py-5 text-center text-sm text-ink-500">No matches</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>
