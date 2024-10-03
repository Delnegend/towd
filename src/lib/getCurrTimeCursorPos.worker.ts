/**
 * A worker that calculates the current time cursor position
 * on the CalendarUI, and trigger an update every 5 minutes
 */

const CELL_HEIGHT = 50;

function timedCount() {
	const today = new Date();
	postMessage({
		// - 100%/7 to get the width of one column
		// getDay(): 0 sun | 1 mon | 2 tue | 3 wed | 4 thu | 5 fri | 6 sat
		// we want: 0 mon | 1 tue | 2 wed | 3 thu | 4 fri | 5 sat | 6 sun
		top: `calc((100%/7)*${today.getDay() === 0 ? 6 : today.getDay() - 1})`,
		left: `${(today.getHours() + today.getMinutes() / 60) * CELL_HEIGHT}px`
	});

	// re-run this function every 5 minutes
	setTimeout(() => {
		timedCount();
	}, 300 * 1000);
}

timedCount();