<script lang="ts">
	import { searchUsers, type UserRef } from '$lib/api';
	import Icon from './Icon.svelte';
	import Avatar from './Avatar.svelte';

	let { value = $bindable(), me }: { value: UserRef; me: UserRef } = $props();

	let open = $state(false);
	let query = $state('');
	let results = $state<UserRef[]>([]);
	let active = $state(0);
	let searchFailed = $state(false);
	let rootEl: HTMLElement | undefined = $state();
	let inputEl: HTMLInputElement | undefined = $state();
	let seq = 0;

	// Debounced search while open.
	$effect(() => {
		if (!open) return;
		const q = query;
		const t = setTimeout(async () => {
			const id = ++seq;
			try {
				const r = await searchUsers(q);
				if (id !== seq) return;
				results = r;
				active = 0;
				searchFailed = false;
			} catch {
				if (id === seq) searchFailed = true;
			}
		}, q ? 150 : 0);
		return () => clearTimeout(t);
	});

	$effect(() => {
		if (open) inputEl?.focus();
	});

	function pick(u: UserRef) {
		value = u;
		open = false;
		query = '';
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = Math.min(active + 1, results.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = Math.max(active - 1, 0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (results[active]) pick(results[active]);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			e.stopPropagation();
			open = false;
		}
	}

	function onWindowClick(e: MouseEvent) {
		if (open && rootEl && !rootEl.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window onclick={onWindowClick} />

<div class="relative" bind:this={rootEl}>
	<button
		type="button"
		onclick={() => (open = !open)}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-label="Requester: {value.name}. Change requester"
		class="flex w-full items-center gap-3 rounded-lg border border-ink-700 bg-ink-950/60 px-3 py-2 text-left transition hover:border-ink-500 focus-visible:border-primary-400/70 focus-visible:ring-3 focus-visible:ring-primary-400/15 focus-visible:outline-none"
	>
		<Avatar name={value.name} src={value.avatarUrl} size={28} />
		<span class="min-w-0 flex-1">
			<span class="block truncate text-sm text-ink-100">
				{value.name}{#if value.id === me.id}<span class="ml-1 text-ink-500">(you)</span>{/if}
			</span>
			<span class="block truncate text-xs text-ink-500">{value.email}</span>
		</span>
		<Icon name="chevron" class={['size-4 text-ink-500 transition', open && 'rotate-180'].join(' ')} />
	</button>

	{#if open}
		<div class="absolute inset-x-0 top-full z-20 mt-1.5 overflow-hidden rounded-xl border border-ink-800 bg-ink-900 shadow-2xl shadow-shade/40">
			<div class="relative border-b border-ink-800">
				<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
				<input
					bind:this={inputEl}
					bind:value={query}
					onkeydown={onKeydown}
					role="combobox"
					aria-controls="requester-results"
					aria-expanded="true"
					aria-activedescendant={results[active] ? `requester-${results[active].id}` : undefined}
					aria-label="Search users"
					placeholder="Search by name or email"
					class="h-10 w-full bg-transparent pr-3 pl-9 text-sm text-ink-100 outline-none placeholder:text-ink-500"
				/>
			</div>
			<ul id="requester-results" role="listbox" class="max-h-64 overflow-y-auto p-1">
				{#each results as u, i (u.id)}
					<!-- Keyboard selection is handled by the search box (aria-activedescendant). -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id="requester-{u.id}"
						role="option"
						aria-selected={u.id === value.id}
						onmousedown={(e) => e.preventDefault()}
						onclick={() => pick(u)}
						onmousemove={() => (active = i)}
						class={['flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2', i === active && 'bg-ink-850']}
					>
						<Avatar name={u.name} src={u.avatarUrl} size={28} />
						<span class="min-w-0 flex-1">
							<span class="block truncate text-sm text-ink-100">
								{u.name}{#if u.id === me.id}<span class="ml-1 text-ink-500">(you)</span>{/if}
							</span>
							<span class="block truncate text-xs text-ink-500">{u.email}</span>
						</span>
						{#if u.id === value.id}
							<Icon name="check" class="size-4 text-primary-400" />
						{/if}
					</li>
				{:else}
					<li class="px-3 py-6 text-center text-sm text-ink-500">
						{searchFailed ? "Couldn't search users." : query ? `No one matches "${query}".` : 'Searching…'}
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>
