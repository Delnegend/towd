let baseURL: URL;

try {
    baseURL = new URL("", window.localStorage.getItem("baseURL") ?? "");
} catch {
    baseURL = new URL(window.location.origin);
}

export { baseURL };
