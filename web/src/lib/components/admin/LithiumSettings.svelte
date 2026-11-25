<script lang="ts">
	import { page } from '$app/state';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import TicketSettings from './TicketSettings.svelte';
	import UserSettings from './UserSettings.svelte';
	import AppearanceSettings from './AppearanceSettings.svelte';
	import EmailSettings from './EmailSettings.svelte';
	import LockdownSettings from './LockdownSettings.svelte';

	// Application-wide configuration, split into sections. Each section is
	// empty for now; real settings get added to the matching block below.
	const sections = [
		{ id: 'tickets', label: 'Tickets', noun: 'ticket', icon: 'ticket', description: 'Defaults and rules for how tickets are raised and worked.' },
		{ id: 'appearance', label: 'Appearance', noun: 'appearance', icon: 'palette', description: 'Your branding, colours and how Lithium looks for everyone.' },
		{ id: 'users', label: 'Users', noun: 'user', icon: 'users', description: 'Sign-in, password rules and defaults for new accounts.' },
		{ id: 'email', label: 'Email', noun: 'email', icon: 'mail', description: 'The mail server Lithium sends notification emails through.' },
		{ id: 'lockdown', label: 'Lockdown', noun: 'lockdown', icon: 'lock', description: 'Emergency mode: sign everyone else out and stop anyone signing in.' }
	] as const satisfies readonly { id: string; label: string; noun: string; icon: IconName; description: string }[];

	type SectionId = (typeof sections)[number]['id'];

	const current = $derived(sections.find((s) => s.id === page.url.searchParams.get('section')) ?? sections[0]);

	function href(id: SectionId) {
		const url = new URL(page.url);
		url.searchParams.set('section', id);
		return url.pathname + url.search;
	}
</script>

<p class="mt-6 max-w-2xl text-sm text-ink-400">
	Configuration for the whole Lithium workspace. Changes here affect everyone.
</p>

<div class="mt-6 grid gap-6 md:grid-cols-[200px_1fr] md:gap-8">
	<!-- Section menu: vertical on desktop, swipeable pills on small screens -->
	<nav aria-label="Settings sections" class="no-scrollbar -mx-4 overflow-x-auto px-4 md:mx-0 md:overflow-visible md:px-0">
		<ul class="flex gap-1 md:flex-col">
			{#each sections as s (s.id)}
				{@const active = current.id === s.id}
				<li class="shrink-0">
					<a
						href={href(s.id)}
						data-sveltekit-replacestate
						data-sveltekit-noscroll
						aria-current={active ? 'page' : undefined}
						class={[
							'flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm whitespace-nowrap transition',
							'focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none',
							active
								? 'bg-ink-850 font-medium text-ink-100'
								: 'text-ink-400 hover:bg-ink-900 hover:text-ink-100'
						]}
					>
						<Icon name={s.icon} class={['size-4', active ? 'text-primary-400' : ''].join(' ')} />
						{s.label}
					</a>
				</li>
			{/each}
		</ul>
	</nav>

	<!-- Section content -->
	<section aria-labelledby="settings-heading" class="min-w-0">
		<h2 id="settings-heading" class="text-base font-semibold tracking-tight">{current.label}</h2>
		<p class="mt-1 text-sm text-ink-400">{current.description}</p>

		{#if current.id === 'tickets'}
			<TicketSettings />
		{:else if current.id === 'users'}
			<UserSettings />
		{:else if current.id === 'appearance'}
			<AppearanceSettings />
		{:else if current.id === 'email'}
			<EmailSettings />
		{:else if current.id === 'lockdown'}
			<LockdownSettings />
		{/if}
	</section>
</div>
