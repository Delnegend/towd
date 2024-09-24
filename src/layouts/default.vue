<template>
	<div>
		<div class="sticky top-0 z-10 bg-white py-3">
			<Tabs :default-value="useRoute().name?.toString()">
				<TabsList class="mx-auto grid max-w-fit grid-cols-3">
					<NuxtLink to="/">
						<TabsTrigger
							value="index"
							class="w-full"
						>
							Authenticate
						</TabsTrigger>
					</NuxtLink>

					<NuxtLink to="/calendar">
						<TabsTrigger
							value="calendar"
							class="w-full"
						>
							Calendar
						</TabsTrigger>
					</NuxtLink>
					<NuxtLink to="/kanban">
						<TabsTrigger
							value="kanban"
							class="w-full"
						>
							Kanban board
						</TabsTrigger>
					</NuxtLink>
				</TabsList>
			</Tabs>
		</div>

		<slot />

		<Toaster />
	</div>
</template>