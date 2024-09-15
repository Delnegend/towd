<script setup lang="ts">
/* eslint-disable no-undef */

const CELL_HEIGHT = 50;

const HOURS = ((): Array<string> => {
	const hours: Array<string> = [];
	for (let i = 0; i < 24; i++) {
		const hour = i % 12 === 0 ? 12 : i % 12;
		const suffix = i < 12 ? "AM" : "PM";
		hours.push(`${hour}${suffix}`);
	}

	return hours;
})();

interface CalendarEvent {
	id: string;
	title: string;
	startDate: Date;
	endDate: Date;
	location: string;
	description: string;

	cssSpaceTop: string;
	cssMaxHeight: string;
}

const calendarEvents: Record<number, Array<CalendarEvent>> = {
	25: [
		{
			id: "1",
			title: "Title",
			location: "Location",
			description: "Description",
			startDate: new Date("2022-01-01T07:00:00"),
			endDate: new Date("2022-01-01T08:00:00"),
			cssSpaceTop: "",
			cssMaxHeight: "",
		},
	],
	31: [
		{
			id: "2",
			title: "Title",
			location: "Location",
			description: "Description",
			startDate: new Date("2022-01-01T09:00:00"),
			endDate: new Date("2022-01-01T10:00:00"),
			cssSpaceTop: "",
			cssMaxHeight: "",
		},

	],
	29: [],
	30: [],
};

const eventHeights: Record<string, number> = {};
for (const [_, eventsInDay] of Object.entries(calendarEvents)) {
	for (const event of eventsInDay) {
		const startHour = event.startDate.getHours() + event.startDate.getMinutes() / 60;
		const endHour = event.endDate.getHours() + event.endDate.getMinutes() / 60;
		eventHeights[event.id] = (endHour - startHour) * CELL_HEIGHT - 2;
	}
}

function getCurrentWeekdays(): Array<Date> {
	const currentDate = new Date();
	const firstDay = currentDate.getDate() - currentDate.getDay() + 1;
	const lastDay = firstDay + 6;

	const weekdays: Array<Date> = [];
	for (let i = firstDay; i <= lastDay; i++) {
		weekdays.push(new Date(currentDate.getFullYear(), currentDate.getMonth(), i));
	}

	return weekdays;
}

// updating the current time cursor position every 5 minutes
// (config the interval in the worker file)
const currTimeCursorPos = reactive({
	top: '',
	left: '',
	ready: false,
});
const worker = new Worker(new URL('../lib/getCurrTimeCursorPos.worker.ts', import.meta.url));
worker.onmessage = (e: MessageEvent<{ top: string; left: string }>) => {
	currTimeCursorPos.top = e.data.top;
	currTimeCursorPos.left = e.data.left;
	currTimeCursorPos.ready = true;
};

// #region - animation handling when hover in/out of each event
function handleMouseEnterEvent(target: EventTarget | null): void {
	if (target === null) {
		return;
	}

	const element = target as HTMLElement;
	element.style.maxHeight = "100vh";
	element.style.transform = "scale(1.01)";
	element.style.zIndex = "10";
}

function handleMouseLeaveEvent(target: EventTarget | null, realHeight: number): void {
	if (target === null) {
		return;
	}

	const element = target as HTMLElement;
	element.style.maxHeight = `${realHeight}px`;
	element.style.transform = "scale(1)";
}

function handleTransitionEnd(target: EventTarget | null) {
	if (target === null) {
		return;
	}

	const element = target as HTMLElement;
	if (element.style.transform !== "scale(1)") {
		return;
	}

	element.style.zIndex = "0";
}
// #endregion
</script>

<template>
	<div class="size-full">
		<!-- Weekdays in text -->
		<div class="sticky top-0 z-20 grid w-full grid-cols-7 border-b border-b-black bg-white pl-20">
			<div
				v-for="(wday, index) in getCurrentWeekdays()"
				:key="index"
				class="group flex flex-col items-center justify-center gap-1 py-4"
				:class="{
					'today': wday.getDate() == new Date().getDate(),
				}"
			>
				<div
					class="w-20 px-4 text-center text-sm text-gray-600 group-[.today]:text-blue-600"
				>
					{{ wday.toLocaleString('en-US', { weekday: 'short' }).slice(0, 3).toUpperCase() }}
				</div>

				<div
					class=" aspect-square rounded-full  p-2 text-center text-2xl font-semibold  text-gray-500 group-[.today]:bg-blue-600 group-[.today]:text-white group-[.today]:shadow-md"
				>
					{{ wday.getDate() }}
				</div>
			</div>
		</div>

		<!-- Wrapper of the rest of the calendar -->
		<div class="flex flex-row">
			<!-- Hours col, flex w/ fixed width -->
			<div class="flex w-20 flex-col">
				<div
					v-for="(item, index) in HOURS"
					:key="index"
					class="flex flex-col items-center justify-center"
					:style="`height: ${CELL_HEIGHT}px`"
				>
					<div class="w-20 -translate-y-8 text-center">
						{{ item }}
					</div>
				</div>
			</div>

			<!-- Cols container -->
			<div class="relative grid w-full grid-cols-7">
				<!-- Each col is a day -->
				<div
					v-for="(day, weekdayIdx) in getCurrentWeekdays()"
					:key="`${day}${weekdayIdx}`"
					class="relative flex flex-col"
				>
					<!-- Each row is an hour -->
					<div
						v-for="hour in HOURS"
						:key="hour"
						class="border-b border-l border-gray-300"
						:style="{
							borderRightWidth: weekdayIdx === 6 ? '1px' : '0px',
							height: `${CELL_HEIGHT}px`,
						}"
					/>

					<!-- Each element is an event -->
					<Popover>
						<PopoverTrigger
							v-for="e in calendarEvents[day.getDate()]"
							:key="e.id"
							class="absolute m-px flex w-[calc(100%-0.5rem)] flex-col justify-start overflow-hidden rounded-md border border-white bg-green-500 px-3 py-2 text-start text-white shadow-md hover:z-10 hover:scale-[1.03] hover:shadow-lg"
							:style="{
								top: `${(e.startDate.getHours() + e.startDate.getMinutes() / 60) * CELL_HEIGHT}px`,
								transitionProperty: 'max-height, transform, box-shadow',
								transitionDuration: '0.3s',
								transitionTimingFunction: 'cubic-bezier(0.4, 0, 0.2, 1)',
								minHeight: `${eventHeights[e.id]}px`,
								maxHeight: `${eventHeights[e.id]}px`,
							}"
							@mouseenter="handleMouseEnterEvent($event.target)"
							@mouseleave="handleMouseLeaveEvent($event.target, eventHeights[e.id])"
							@transitionend="handleTransitionEnd($event.target)"
						>
							<span class="font-bold">{{ e.title }}</span>
							<div class="text-sm">
								{{ e.startDate.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true }) }}
								-
								{{ e.endDate.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true }) }}
							</div>
							<span>{{ e.description }}</span>
							<span>{{ e.location }}</span>
						</PopoverTrigger>
						<PopoverContent
							side="right"
							align="start"
						>
							Test
						</PopoverContent>
					</Popover>
				</div>

				<div
					class="absolute flex h-[2px] w-[calc(100%/7)] bg-red-500 before:size-3 before:-translate-x-1/2 before:translate-y-[-5px] before:rounded-full before:bg-red-500 before:content-['']"
					:style="{
						opacity: currTimeCursorPos.ready ? '1' : '0',
						left: currTimeCursorPos.top,
						top: currTimeCursorPos.left,
						transition: 'opacity 0.3s',
					}"
				/>
			</div>
		</div>
	</div>
</template>
