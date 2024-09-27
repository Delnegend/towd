// updating the current time cursor position every 5 minutes
const currTimeCursorPos = reactive({
	top: '',
	left: '',
	ready: false,
});

// (config the interval in the worker file)
const worker = new Worker(new URL('../lib/getCurrTimeCursorPos.worker.ts', import.meta.url));
worker.onmessage = (e: MessageEvent<{ top: string; left: string }>) => {
	currTimeCursorPos.top = e.data.top;
	currTimeCursorPos.left = e.data.left;
	currTimeCursorPos.ready = true;
};

export { currTimeCursorPos };
