<script lang="ts" module>
</script>

<script lang="ts">
	import { tick } from 'svelte';
	import Avatar from './Avatar.svelte';
	import Icon from './Icon.svelte';

	/** Someone (or a group) that can be mentioned. `target` is what's stored: "12" or "g:3". */
	export type MentionCandidate =
		| { kind: 'user'; target: string; name: string; email?: string; avatarUrl?: string }
		| { kind: 'group'; target: string; name: string; memberCount: number };

	// A textarea where typing "@" suggests people to mention. Picking someone
	// inserts "@Their Name"; the parent turns that into a stored mention on send.
	let {
		value = $bindable(''),
		load,
		onmention,
		onkeydown,
		id,
		placeholder = '',
		maxlength,
		enabled = true,
		class: cls = ''
	}: {
		value?: string;
		/** Who can be mentioned. Called once, when "@" is first typed. */
		load: () => Promise<MentionCandidate[]>;
		onmention: (name: string, target: string) => void;
		onkeydown?: (e: KeyboardEvent) => void;
		id: string;
		placeholder?: string;
		maxlength?: number;
		/** When false it's a plain textarea: typing @ suggests no one. */
		enabled?: boolean;
		class?: string;
	} = $props();

	let ta: HTMLTextAreaElement | undefined = $state();
	let people = $state<MentionCandidate[] | null>(null);
	let open = $state(false);
	let query = $state('');
	let start = -1; // index of the "@" being completed
	let active = $state(0);

	// A different list of people (e.g. switching to an internal note) starts fresh.
	$effect(() => {
		load;
		people = null;
	});

	const matches = $derived(
		(people ?? []).filter((p) => p.name.toLowerCase().includes(query.toLowerCase())).slice(0, 6)
	);

	async function detect() {
		if (!ta || !enabled) return;
		const pos = ta.selectionStart;
		const m = value.slice(0, pos).match(/(?:^|\s)@([^\s@]{0,30})$/);
		if (!m) {
			open = false;
			return;
		}
		start = pos - m[1].length - 1;
		query = m[1];
		active = 0;
		open = true;
		if (people === null) {
			try {
				people = await load();
			} catch {
				people = [];
			}
		}
	}

	async function pick(p: MentionCandidate) {
		if (!ta) return;
		const end = ta.selectionStart;
		value = `${value.slice(0, start)}@${p.name} ${value.slice(end)}`;
		onmention(p.name, p.target);
		open = false;
		await tick();
		const caret = start + p.name.length + 2;
		ta.setSelectionRange(caret, caret);
		ta.focus();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (open && matches.length > 0) {
			if (e.key === 'ArrowDown') {
				active = (active + 1) % matches.length;
			} else if (e.key === 'ArrowUp') {
				active = (active - 1 + matches.length) % matches.length;
			} else if (e.key === 'Enter' || e.key === 'Tab') {
				pick(matches[active]);
			} else if (e.key === 'Escape') {
				open = false;
			} else {
				onkeydown?.(e);
				return;
			}
			e.preventDefault();
			return;
		}
		onkeydown?.(e);
	}
</script>

<div class="relative">
	<textarea
		bind:this={ta}
		bind:value
		{id}
		{placeholder}
		{maxlength}
		oninput={detect}
		onclick={detect}
		onkeydown={handleKeydown}
		onblur={() => setTimeout(() => (open = false), 150)}
		role="combobox"
		aria-expanded={open && matches.length > 0}
		aria-controls="{id}-mentions"
		aria-autocomplete="list"
		aria-activedescendant={open && matches[active] ? `${id}-mention-${active}` : undefined}
		class={cls}
	></textarea>

	{#if open && people !== null}
		<ul
			id="{id}-mentions"
			role="listbox"
			aria-label="People to mention"
			class="absolute bottom-full left-0 z-30 mb-2 w-72 max-w-full overflow-hidden rounded-xl border border-ink-800 bg-ink-900 p-1.5 shadow-2xl shadow-shade/40"
		>
			{#each matches as p, i (p.target)}
				<!-- Keyboard selection is handled by the textarea (aria-activedescendant). -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="{id}-mention-{i}"
					role="option"
					aria-selected={i === active}
					onmousedown={(e) => e.preventDefault()}
					onclick={() => pick(p)}
					onmousemove={() => (active = i)}
					class={['flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5', i === active && 'bg-ink-850']}
				>
					{#if p.kind === 'group'}
						<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary-400/15 text-primary-300">
							<Icon name="users" class="size-3.5" />
						</span>
						<span class="min-w-0">
							<span class="block truncate text-sm text-ink-100">{p.name}</span>
							<span class="block truncate text-xs text-ink-500">
								Group · notifies {p.memberCount} {p.memberCount === 1 ? 'member' : 'members'}
							</span>
						</span>
					{:else}
						<Avatar name={p.name} src={p.avatarUrl} size={24} />
						<span class="min-w-0">
							<span class="block truncate text-sm text-ink-100">{p.name}</span>
							{#if p.email}<span class="block truncate text-xs text-ink-500">{p.email}</span>{/if}
						</span>
					{/if}
				</li>
			{:else}
				<li class="px-3 py-3 text-center text-sm text-ink-500">
					{query ? `No one called “${query}” can see this ticket` : 'No one else to mention'}
				</li>
			{/each}
		</ul>
	{/if}
</div>
