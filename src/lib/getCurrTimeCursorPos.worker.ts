/**
 * A worker that calculates the current time cursor position
 * on the CalendarUI, and trigger an update every 5 minutes
 */

const CELL_HEIGHT = 50;

function timedCount() {
	const today = new Date();
	postMessage({
		// - 100%/7 to get the width of one column
		// - today.getDay() to get the current column index
		top: `calc((100%/7)*${today.getDay() - 1})`,
		left: `${(today.getHours() + today.getMinutes() / 60) * CELL_HEIGHT}px`
	});

	// re-run this function every 5 minutes
	setTimeout(() => {
		timedCount();
	}, 300 * 1000);
}

timedCount();