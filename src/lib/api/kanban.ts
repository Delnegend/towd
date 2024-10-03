/* eslint-disable @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-argument */

export interface KanbanItemReqBody {
	id: number;
	content: string;
}

export interface KanbanGroupReqBody {
	groupName: string;
	items: Array<KanbanItemReqBody>;
}

export interface KanbanTableReqRespBody {
	tableName: string;
	groups: Array<KanbanGroupReqBody>;
}

async function LoadKanbanTable(): Promise<KanbanTableReqRespBody> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/kanban/load", import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return "/kanban/load";
	})()
	const resp = await fetch(endpoint, {
		method: "GET",
		headers: { "Content-Type": "application/json" },
		credentials: import.meta.dev ? 'include' : 'same-origin',
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}

	return (await resp.json()) as KanbanTableReqRespBody;
}

async function SaveKanbanTable(data: KanbanTableReqRespBody): Promise<void> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/kanban/save", import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return "/kanban/save";
	})();
	const resp = await fetch(endpoint, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		credentials: import.meta.dev ? 'include' : 'same-origin',
		body: JSON.stringify(data),
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}
}

export { LoadKanbanTable, SaveKanbanTable };
