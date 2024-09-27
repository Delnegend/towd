/* eslint-disable @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-argument */

export interface GetEventsReqBody {
	startDateUnixUTC: number;
	endDateUnixUTC: number;
}

export interface OneEventRespBody {
	id: string;
	title: string;
	description?: string;
	location?: string;
	url?: string;
	organizer?: string;
	startDateUnixUTC: number;
	endDateUnixUTC: number;
	isWholeDay?: boolean;
}

/** Get all events in a date range. */
async function GetEvents(data: GetEventsReqBody): Promise<Array<OneEventRespBody>> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/calendar/get-events", import.meta.env.VITE_SERVER_HOSTNAME ?? "");
		}

		return "/calendar/get-events";
	})();
	const resp_ = await fetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
			...(import.meta.dev ? { 'Authorization': `Bearer ${  window.localStorage.getItem("sessionSecret")}` } : {}),
		},
		body: JSON.stringify(data),
	});
	if (!resp_.ok) {
		throw new Error(`${resp_.status} ${(await resp_.text()).slice(0, 200)}`);
	}

	return (await resp_.json()) as Array<OneEventRespBody>;
}

export interface CreateEventReqBody {
	title: string;
	description: string;
	location: string;
	url: string;
	organizer: string;
	startDateUnixUTC: number;
	endDateUnixUTC: number;
}

/** Create a new event, the success response is the event ID. */
async function CreateEvent(data: CreateEventReqBody): Promise<string> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/calendar/create-event", import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return "/calendar/create-event";
	})();
	const resp = await fetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
			...(import.meta.dev ? { 'Authorization': `Bearer ${  window.localStorage.getItem("sessionSecret")}` } : {}),
		},
		body: JSON.stringify(data),
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}

	return await resp.text();
}

export type ModifyEventReqBody = CreateEventReqBody & {
	id: string;
};

/** Modify an existing event. */
async function ModifyEvent(data: ModifyEventReqBody): Promise<void> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL(`/calendar/modify-event/${data.id}`, import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return `/calendar/modify-event/${data.id}`;
	})();
	const resp = await fetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(data),
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}
}

async function DeleteEvent(id: string): Promise<void> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL(`/event/${id}`, import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return `/event/${id}`;
	})();
	const resp_ = await fetch(endpoint, {
		method: "DELETE",
		headers: {
			...(import.meta.dev ? { 'Authorization': `Bearer ${  window.localStorage.getItem("sessionSecret")}` } : {}),
		},
	});
	if (!resp_.ok) {
		throw new Error(`${resp_.status} ${(await resp_.text()).slice(0, 200)}`);
	}
}

export { CreateEvent, DeleteEvent, GetEvents, ModifyEvent };
