import { baseURL } from "./base";

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
	let resp: Response;
	try {
		const resp_ = await fetch(new URL("/calendar/get-events", baseURL), {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(data),
		});
		if (!resp_.ok) {
			const err: string = await resp_.text();
			return await Promise.reject(new Error(`GetEventsError: [${resp_.status}] ${err}`));
		}

		resp = resp_;
	} catch (err) {
		return await Promise.reject(new Error(`GetEventsError: ${err}`));
	}

	try {
		const respBody = (await resp.json()) as Array<OneEventRespBody>;
		if (respBody.length === 0) {
			return await Promise.reject(new Error("GetEventsError: response body is empty"));
		}

		return respBody;
	} catch (err) {
		return await Promise.reject(new Error(`GetEventsError: can't parse response body: ${err}`));
	}
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
	let resp: Response;
	try {
		const resp_ = await fetch(new URL("/calendar/create-event", baseURL), {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(data),
		});
		if (!resp_.ok) {
			return await Promise.reject(new Error(`CreateEventError: [${resp_.status}] ${await resp_.text()}`));
		}

		resp = resp_;
	} catch (err) {
		return await Promise.reject(new Error(`CreateEventError: ${err}`));
	}

	try {
		return (await resp.json()) as string;
	} catch (err) {
		return await Promise.reject(new Error(`CreateEventError: can't parse response body: ${err}`));
	}
}

export type ModifyEventReqBody = CreateEventReqBody & {
	id: string;
};

/** Modify an existing event. */
async function ModifyEvent(data: ModifyEventReqBody): Promise<void> {
	try {
		const resp_ = await fetch(new URL(`/calendar/modify-event/${data.id}`, baseURL), {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(data),
		});
		if (!resp_.ok) {
			await Promise.reject(new Error(`ModifyEventError: [${resp_.status}] ${await resp_.text()}`));
		}
	} catch (err) {
		await Promise.reject(new Error(`ModifyEventError: ${err}`));
	}
}

async function DeleteEvent(id: string): Promise<void> {
	try {
		const resp_ = await fetch(new URL(`/event/${id}`, baseURL), {
			method: "DELETE",
		});
		if (!resp_.ok) {
			await Promise.reject(new Error(`DeleteEventError: [${resp_.status}] ${await resp_.text()}`));
		}
	} catch (err) {
		await Promise.reject(new Error(`DeleteEventError: ${err}`));
	}
}

export { CreateEvent, DeleteEvent, GetEvents, ModifyEvent };
