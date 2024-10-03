/* eslint-disable @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-argument */

async function Login(tempKey: string): Promise<void> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/auth", import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return "/auth";
	})();
	const resp = await fetch(endpoint, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify({
			tempKey,
		}),
		credentials: import.meta.dev ? 'include' : 'same-origin',
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}
}

async function Logout(): Promise<void> {
	const endpoint = (() => {
		if (import.meta.dev) {
			// @ts-expect-error - env do exist
			return new URL("/auth", import.meta.env.VITE_SERVER_HOSTNAME);
		}

		return "/auth";
	})();
	const resp = await fetch(endpoint, {
		method: 'DELETE',
		credentials: import.meta.dev ? 'include' : 'same-origin',
	});
	if (!resp.ok) {
		throw new Error(`${resp.status} ${(await resp.text()).slice(0, 200)}`);
	}
}

export { Login, Logout };
