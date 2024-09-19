import { baseURL } from "./base";

interface KanbanItemReqBody {
    id: number;
    content: string;
}

interface KanbanGroupReqBody {
    groupName: string;
    items: Array<KanbanItemReqBody>;
}

interface KanbanTableReqBody {
    tableName: string;
    groups: Array<KanbanGroupReqBody>;
}

async function GetKanbanTable(): Promise<KanbanTableReqBody> {
    let resp: Response;
    try {
        resp = await fetch(new URL("/kanban/get-groups", baseURL), {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
            },
        });
        if (!resp.ok) {
            return await Promise.reject(new Error(`GetKanbanTableError: [${resp.status}] ${await resp.text()}`));
        }
    } catch (err) {
        return Promise.reject(new Error(`GetKanbanTableError: ${err}`));
    }

    try {
        return (await resp.json()) as KanbanTableReqBody;
    } catch (err) {
        return Promise.reject(new Error(`GetKanbanTableError: can't parse response body: ${err}`));
    }
}

interface CreateKanbanItemReqBody {
    groupName: string;
    content: string;
}

async function CreateKanbanItem(data: CreateKanbanItemReqBody): Promise<string> {
    let resp: Response;
    try {
        resp = await fetch(new URL("/kanban/create-item", baseURL), {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(data),
        });
        if (!resp.ok) {
            return await Promise.reject(new Error(`CreateKanbanItemError: [${resp.status}] ${await resp.text()}`));
        }
    } catch (err) {
        return Promise.reject(new Error(`CreateKanbanItemError: ${err}`));
    }

    try {
        return (await resp.json()) as string;
    } catch (err) {
        return Promise.reject(new Error(`CreateKanbanItemError: can't parse response body: ${err}`));
    }
}

async function DeleteKanbanItem(id: string): Promise<void> {
    try {
        const resp_ = await fetch(new URL(`/kanban/delete-item/${id}`, baseURL), {
            method: "DELETE",
        });
        if (!resp_.ok) {
            await Promise.reject(new Error(`DeleteKanbanItemError: [${resp_.status}] ${await resp_.text()}`));
        }
    } catch (err) {
        await Promise.reject(new Error(`DeleteKanbanItemError: ${err}`));
    }
}

export { CreateKanbanItem, DeleteKanbanItem, GetKanbanTable };
