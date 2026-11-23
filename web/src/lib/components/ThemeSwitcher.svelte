<script lang="ts">
	import { theme, type ThemePreference } from '$lib/theme.svelte';
	import Icon, { type IconName } from './Icon.svelte';

	let { showLabels = false }: { showLabels?: boolean } = $props();

	const options: { value: ThemePreference; label: string; icon: IconName }[] = [
		{ value: 'light', label: 'Light', icon: 'sun' },
		{ value: 'dark', label: 'Dark', icon: 'moon' },
		{ value: 'system', label: 'System', icon: 'monitor' }
	];
</script>

<div
	role="radiogroup"
	aria-label="Theme"
	class="inline-flex items-center gap-0.5 rounded-lg border border-ink-800 bg-ink-950/60 p-0.5"
>
	{#each options as opt (opt.value)}
		{@const active = theme.preference === opt.value}
		<button
			type="button"
			role="radio"
			aria-checked={active}
			aria-label={opt.label}
			title={opt.label}
			onclick={() => theme.set(opt.value)}
			class={[
				'flex h-7 items-center justify-center gap-1.5 rounded-md text-xs font-medium transition',
				'focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none',
				showLabels ? 'flex-1 px-2' : 'w-7',
				active ? 'bg-ink-850 text-ink-100 shadow-sm shadow-shade/20' : 'text-ink-500 hover:text-ink-200'
			]}
		>
			<Icon name={opt.icon} class={['size-3.5', active ? 'text-primary-400' : ''].join(' ')} />
			{#if showLabels}{opt.label}{/if}
		</button>
	{/each}
</div>
