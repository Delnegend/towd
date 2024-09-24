export const baseURL = (() => {
	try {
		return new URL("", window.localStorage.getItem("baseURL") ?? "");
	} catch {
		return new URL(window.location.origin);
	}
})();