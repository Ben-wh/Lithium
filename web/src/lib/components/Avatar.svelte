<script lang="ts">
	import { initials } from '$lib/tickets';

	// A user's profile picture, falling back to their initials when they
	// haven't set one (or a custom picture is unavailable, e.g. disabled by an admin).
	let {
		name,
		src = '',
		size = 32,
		tone = 'neutral',
		class: cls = ''
	}: {
		name: string;
		src?: string | null;
		size?: number;
		/** Colour of the initials fallback. */
		tone?: 'neutral' | 'soft' | 'primary';
		class?: string;
	} = $props();

	let failed = $state(false);
	$effect(() => {
		src; // a new picture gets a fresh chance to load
		failed = false;
	});

	const tones = {
		neutral: 'bg-ink-800 text-ink-200',
		soft: 'bg-primary-400/15 text-primary-200',
		primary: 'bg-primary-400 text-primary-950'
	};
</script>

{#if src && !failed}
	<img
		{src}
		alt=""
		width={size}
		height={size}
		loading="lazy"
		onerror={() => (failed = true)}
		class={['shrink-0 rounded-full bg-ink-800 object-cover', cls]}
		style:width="{size}px"
		style:height="{size}px"
	/>
{:else}
	<span
		aria-hidden="true"
		class={['flex shrink-0 items-center justify-center rounded-full font-semibold select-none', tones[tone], cls]}
		style:width="{size}px"
		style:height="{size}px"
		style:font-size="{Math.max(8, Math.round(size * 0.36))}px"
	>
		{initials(name) || '?'}
	</span>
{/if}
