<script lang="ts">
	import { untrack } from 'svelte';
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import { EMAIL_EVENTS, profile, type EmailEvent, type User } from '$lib/api';
	import Icon from './Icon.svelte';
	import Toggle from './Toggle.svelte';

	// The signed-in user's own choices about notification emails. Hidden when
	// the server can't send email; read-only when an admin has turned it off.
	let { user }: { user: User } = $props();

	const configured = $derived(page.data.settings?.emailConfigured ?? false);
	const workspaceOn = $derived(page.data.settings?.emailNotifications ?? false);
	const staff = $derived(user.role === 'admin' || user.role === 'agent');

	const labels: Record<EmailEvent, { label: string; description: () => string }> = {
		replies: {
			label: 'Replies',
			description: () => (staff ? 'New replies and internal notes.' : 'New replies on your tickets.')
		},
		mentions: {
			label: 'Mentions',
			description: () => (staff ? "Someone @mentions you or a group you're in." : 'Someone @mentions you.')
		},
		assigned: { label: 'Assigned to you', description: () => 'A ticket is assigned to you.' },
		updates: { label: 'Status and other changes', description: () => 'Status, priority, category or assignee changes.' }
	};
	// Only staff can be assigned tickets, so others don't see that choice.
	const shown = $derived(EMAIL_EVENTS.filter((e) => e !== 'assigned' || staff));

	// Start from the saved choices, then follow the controls.
	let enabled = $state(untrack(() => user.emailNotifications));
	let events = $state<EmailEvent[]>(untrack(() => [...user.emailEvents]));
	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let statusTimer: ReturnType<typeof setTimeout>;

	async function save(patch: { enabled?: boolean; events?: EmailEvent[] }, revert: () => void) {
		saving = true;
		clearTimeout(statusTimer);
		try {
			await profile.setEmailPreferences(patch);
			await invalidate('/api/auth/me'); // keep the signed-in user up to date
			status = 'saved';
		} catch {
			revert();
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
		}
	}

	function toggleEvent(e: EmailEvent, on: boolean) {
		const previous = events;
		events = on ? EMAIL_EVENTS.filter((x) => x === e || events.includes(x)) : events.filter((x) => x !== e);
		save({ events }, () => (events = previous));
	}
</script>

{#if configured}
	<section class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60 p-6">
		<div class="mb-4 flex items-center justify-between gap-3">
			<h2 class="text-sm font-semibold">Email notifications</h2>
			<span class="shrink-0 text-xs" aria-live="polite">
				{#if saving}
					<span class="text-ink-500">Saving…</span>
				{:else if status === 'saved'}
					<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
				{:else if status === 'error'}
					<span class="text-danger-300">Couldn't save. Try again.</span>
				{/if}
			</span>
		</div>
		{#if workspaceOn}
			<Toggle bind:checked={enabled} disabled={saving} onchange={(on) => save({ enabled: on }, () => (enabled = !on))}>
				Email me about my tickets
				{#snippet description()}
					Sent to {user.email}. Several updates to a ticket within a couple of minutes arrive as one email. You'll
					still see everything in the app.
				{/snippet}
			</Toggle>
			{#if enabled}
				<fieldset class="mt-5 border-t border-ink-800 pt-4">
					<legend class="sr-only">Email me about</legend>
					<p class="mb-3 text-[13px] font-medium text-ink-200">Email me about</p>
					<div class="space-y-3">
						{#each shown as e (e)}
							<label class="flex cursor-pointer items-start gap-3">
								<input
									type="checkbox"
									checked={events.includes(e)}
									disabled={saving}
									onchange={(ev) => toggleEvent(e, ev.currentTarget.checked)}
									class="mt-0.5 size-4 shrink-0 cursor-pointer accent-[var(--color-primary-400)]"
								/>
								<span>
									<span class="block text-sm text-ink-100">{labels[e].label}</span>
									<span class="block text-sm text-ink-400">{labels[e].description()}</span>
								</span>
							</label>
						{/each}
					</div>
					{#if !shown.some((e) => events.includes(e))}
						<p class="mt-3 text-xs text-ink-500">Nothing selected, so you won't get any emails.</p>
					{/if}
				</fieldset>
			{/if}
		{:else}
			<!-- Shown as off, since no emails are sent; the person's own choice is kept for when it's back on. -->
			<Toggle checked={false} disabled>
				Email me about my tickets
				{#snippet description()}
					Your administrator has turned off email notifications for everyone.
				{/snippet}
			</Toggle>
		{/if}
	</section>
{/if}
