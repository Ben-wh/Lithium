import { adminTemplates, type TicketTemplate } from '$lib/api';

// Ticket templates, shared by the template and category managers so linking
// a category and editing a template stay in step on the same page.
export const templateStore = $state<{ list: TicketTemplate[]; loaded: boolean; error: string }>({
	list: [],
	loaded: false,
	error: ''
});

export async function loadTemplates() {
	try {
		templateStore.list = await adminTemplates.list();
		templateStore.error = '';
	} catch {
		templateStore.error = "Couldn't load ticket templates.";
	} finally {
		templateStore.loaded = true;
	}
}
