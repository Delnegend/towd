/** Get the beginning and end of the current week in UTC Unix timestamps */
export function getWeekUTCTimestamps(): { startOfWeekUTCTimestamp: number; endOfWeekUTCTimestamp: number } {
	const now = new Date();

	// Get the current day of the week (0 = Sunday, 1 = Monday, ..., 6 = Saturday)
	const dayOfWeek = now.getUTCDay();

	// Calculate the difference to the previous Monday
	const diffToMonday = (dayOfWeek === 0 ? -6 : 1) - dayOfWeek;

	// Get the start of the week (Monday)
	const startOfWeek = new Date(now);
	startOfWeek.setUTCDate(now.getUTCDate() + diffToMonday);
	startOfWeek.setUTCHours(0, 0, 0, 0);

	// Get the end of the week (Sunday)
	const endOfWeek = new Date(startOfWeek);
	endOfWeek.setUTCDate(startOfWeek.getUTCDate() + 6);
	endOfWeek.setUTCHours(23, 59, 59, 999);

	// Convert to Unix timestamps
	const startOfWeekUTCTimestamp = Math.floor(startOfWeek.getTime() / 1000);
	const endOfWeekUTCTimestamp = Math.floor(endOfWeek.getTime() / 1000) + 1;

	return {
		startOfWeekUTCTimestamp,
		endOfWeekUTCTimestamp
	};
}