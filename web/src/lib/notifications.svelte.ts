import { notificationsApi } from './api';

// The signed-in user's unread notification count, shared by the bell in the
// navbar and pages that mark notifications read (e.g. opening a ticket).
class Notifications {
	unread = $state(0);

	async refresh() {
		try {
			this.unread = (await notificationsApi.unread()).unread;
		} catch {
			/* offline or signed out; try again next time */
		}
	}

	async markTicketRead(ticketId: number) {
		try {
			this.unread = (await notificationsApi.markRead({ ticketId })).unread;
		} catch {
			/* not critical */
		}
	}
}

export const notifications = new Notifications();
