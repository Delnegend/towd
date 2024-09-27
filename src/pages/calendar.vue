<script setup lang="ts">
import { toast } from 'vue-sonner';
import { currTimeCursorPos } from "~/composables/page.calendar.state";
import { GetEvents, type OneEventRespBody } from '~/lib/api';
import { getWeekUTCTimestamps } from '~/lib/getWeekUTCTimestamp';

const CELL_HEIGHT = 50;
const events = ref<Array<OneEventRespBody>>([]);
const { startOfWeekUTCTimestamp, endOfWeekUTCTimestamp } = getWeekUTCTimestamps();

onMounted(async () => {
	try {
		events.value = await GetEvents({ startDateUnixUTC: startOfWeekUTCTimestamp, endDateUnixUTC: endOfWeekUTCTimestamp });
	} catch (error) {
		toast.error("Can't fetch events", {
			description: `${error}`,
		});
	}
});

interface ProcessedEvent {
	id: string;
	title: string;
	description?: string;
	location?: string;
	url?: string;
	organizer?: string;
	startDate: Date;
	endDate: Date;

	cssSpaceTop: string;
	cssEventHeight: string;
}
const processedEvents = ref<Record<number, Array<ProcessedEvent>>>({});
watch(events, () => {
	const result: Record<number, Array<ProcessedEvent>> = {};
	for (const event of events.value) {
		const startDate = new Date(event.startDateUnixUTC * 1000);
		const endDate = new Date(event.endDateUnixUTC * 1000);

		const startHour = startDate.getHours() + startDate.getMinutes() / 60;
		const endHour = endDate.getHours() + endDate.getMinutes() / 60;
		const eventHeight = (endHour - startHour) * CELL_HEIGHT - 2;
		const spaceTop = (startDate.getHours() + startDate.getMinutes() / 60) * CELL_HEIGHT;

		result[startDate.getDate()].push({
			id: event.id,
			title: event.title,
			description: event.description,
			location: event.location,
			url: event.url,
			organizer: event.organizer,
			startDate,
			endDate,
			cssSpaceTop: `${spaceTop}px`,
			cssEventHeight: `${eventHeight}px`,
		});
	}

	processedEvents.value = result;
});

const currentWeekdays: Array<Date> = (() => {
	const currentDate = new Date();
	const firstDay = currentDate.getDate() - currentDate.getDay() + 1;
	const lastDay = firstDay + 6;

	const weekdays: Array<Date> = [];
	for (let i = firstDay; i <= lastDay; i++) {
		weekdays.push(new Date(currentDate.getFullYear(), currentDate.getMonth(), i));
	}

	return weekdays;
})();

const hourStrings = ((): Array<string> => {
	const hours: Array<string> = [];
	for (let i = 0; i < 24; i++) {
		const hour = i % 12 === 0 ? 12 : i % 12;
		const suffix = i < 12 ? "AM" : "PM";
		hours.push(`${hour}${suffix}`);
	}

	return hours;
})();

function handleMouseEnterEvent(target: EventTarget | null): void {
	if (target === null) {
		return;
	}

	const element = target as HTMLElement;
	element.style.maxHeight = "100vh";
	element.style.transform = "scale(1.01)";
	element.style.zIndex = "10";
}

function handleMouseLeaveEvent(target: EventTarget | null, eventHeight: string): void {
	if (target === null) {
		return;
	}

	const element = target as HTMLElement;
	element.style.maxHeight = eventHeight;
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
</script>

<template>
	<div class="size-full">
		<!-- Weekdays in text -->
		<div class="sticky top-16 z-20 grid w-full grid-cols-7 border-b border-b-black bg-white pl-20">
			<div
				v-for="(wday, index) in currentWeekdays"
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
					v-for="(item, index) in hourStrings"
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
					v-for="(day, weekdayIdx) in currentWeekdays"
					:key="`${day}${weekdayIdx}`"
					class="relative flex flex-col"
				>
					<!-- Each row is an hour -->
					<div
						v-for="hour in hourStrings"
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
							v-for="e in processedEvents[day.getDate()]"
							:key="e.id"
							class="absolute m-px flex w-[calc(100%-0.5rem)] flex-col justify-start overflow-hidden rounded-md border border-white bg-green-500 px-3 py-2 text-start text-white shadow-md hover:z-10 hover:scale-[1.03] hover:shadow-lg"
							:style="{
								top: e.cssSpaceTop,
								transitionProperty: 'max-height, transform, box-shadow',
								transitionDuration: '0.3s',
								transitionTimingFunction: 'cubic-bezier(0.4, 0, 0.2, 1)',
								minHeight: e.cssEventHeight,
								maxHeight: e.cssEventHeight,
							}"
							@mouseenter="handleMouseEnterEvent($event.target)"
							@mouseleave="handleMouseLeaveEvent($event.target, e.cssEventHeight)"
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