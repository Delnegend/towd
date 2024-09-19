import { baseURL } from "./base";

async function login(tempKey: string): Promise<void> {
    try {
        const resp = await fetch(new URL(`/auth`, baseURL), {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                tempKey,
            }),
        });
        if (!resp.ok) {
            await Promise.reject(new Error(`${resp.status} ${await resp.text()}`));
        }
    } catch (error) {
        await Promise.reject(new Error(`loginError: ${error}`));
    }
}

async function logout(tempKey: string): Promise<void> {
    try {
        const resp = await fetch(new URL(`/auth`, baseURL), {
            method: 'DELETE',
        });
        if (!resp.ok) {
            await Promise.reject(new Error(`${resp.status} ${await resp.text()}`));
        }
    } catch (error) {
        return Promise.reject(new Error(`logoutError: ${error}`));
    }
}

export { login, logout };
