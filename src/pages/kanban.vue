<script setup lang="ts">
import { Pencil, Trash2 } from 'lucide-vue-next';
import { toast } from 'vue-sonner';
import draggable from 'vuedraggable';
import { LoadKanbanTable, SaveKanbanTable, type KanbanTableReqRespBody } from '~/lib/api';

const KanbanBoard: Ref<KanbanTableReqRespBody> = ref({
	tableName: "",
	groups: [],
});

onMounted(async () => {
	try {
		KanbanBoard.value = await LoadKanbanTable();
		// eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
		if (KanbanBoard.value.groups === null) {
			KanbanBoard.value.groups = [];
		}
	} catch (error) {
		toast.error("Can't fetch kanban board", {
			description: `${error}`,
		});
	}
});

const SaveKanbanTableDebounced = pDebounce(async () => {
	if (KanbanBoard.value.groups.length === 0) {
		return;
	}

	try {
		await SaveKanbanTable(KanbanBoard.value);
	} catch (error) {
		toast.error("Can't save kanban board", {
			description: `${error}`,
		});
	}
}, 1000);

async function AddItem(e: MouseEvent, groupName: string) {
	const newItem = { id: Number(new Date()), content: "New Item" };
	KanbanBoard.value.groups.find((group) => group.groupName === groupName)!.items.push(newItem);
	await SaveKanbanTableDebounced();
}

const modifyItemTextAreaValue = ref("");

async function ModifyItem(itemId: number, groupName: string) {
	const content = modifyItemTextAreaValue.value.trim();
	if (content === "") {
		toast.warning("Content is empty");
		return;
	}

	const item = KanbanBoard.value.groups.find((group) => group.groupName === groupName)!.items.find((item) => item.id === itemId)!;
	item.content = content;
	modifyItemTextAreaValue.value = "";
	await SaveKanbanTableDebounced();
}

async function DeleteItem(itemId: number, groupName: string) {
	KanbanBoard.value.groups.find((group) => group.groupName === groupName)!.items = KanbanBoard.value.groups.find((group) => group.groupName === groupName)!.items.filter((item) => item.id !== itemId);
	await SaveKanbanTableDebounced();
}

const createGroupInputValue = ref("");

async function AddGroup() {
	if (createGroupInputValue.value.trim() === "") {
		toast.warning("Group name is empty");
		return;
	}

	if (KanbanBoard.value.groups.find((group) => group.groupName === createGroupInputValue.value.trim())) {
		toast.warning("Group already exists");
		return;
	}

	if (KanbanBoard.value.groups.length > 0) {
		KanbanBoard.value.groups.push({ groupName: createGroupInputValue.value.trim(), items: [] });
	} else {
		KanbanBoard.value.groups = [{ groupName: createGroupInputValue.value.trim(), items: [] }];
	}

	createGroupInputValue.value = "";
	await SaveKanbanTableDebounced();
}

async function DeleteGroup(groupName: string) {
	// check if group is not empty
	if (KanbanBoard.value.groups.find((group) => group.groupName === groupName)!.items.length > 0) {
		toast.error("Can't delete group, it's not empty");
		return;
	}

	for (const group of KanbanBoard.value.groups) {
		if (group.groupName === groupName) {
			KanbanBoard.value.groups = KanbanBoard.value.groups.filter((group) => group.groupName !== groupName);
			break;
		}
	}

	await SaveKanbanTableDebounced();
}

</script>

<template>
	<div class="h-[calc(100vh-64px)]">
		<div class="flex h-full flex-row justify-start gap-x-5 overflow-x-scroll px-5">
			<div v-for="(group, groupIndex) in KanbanBoard.groups" :key="group.groupName" class="flex h-full min-w-80 shrink flex-col rounded-lg border px-4 py-3 text-slate-700 shadow-lg">

				<div class="mb-3 flex flex-row justify-between">
					<span class="text-xl font-bold">{{ group.groupName }}</span>
					<button v-if="groupIndex !== 0" class="text-slate-600 transition-transform hover:scale-110" @click="DeleteGroup(group.groupName)">
						<Trash2 :size="18" />
					</button>
				</div>

				<!-- because we don't want the title to be a part of the sortable -->
				<draggable v-model="group.items" group="grouped" item-key="id" class="flex h-full shrink flex-col gap-2 overflow-y-auto overflow-x-hidden" :animation="150">
					<template #item="{ element }">
						<div class="cursor-grab text-balance rounded-sm border border-slate-400 bg-white px-3 py-2 text-black">
							{{ element.content }}

							<!-- Edit & delete buttons -->
							<div class="flex flex-row justify-end gap-2">
								<Sheet>
									<SheetTrigger as-child>
										<button class="text-slate-600 transition-transform hover:scale-110" @click="modifyItemTextAreaValue = element.content">
											<Pencil :size="18" />
										</button>
									</SheetTrigger>
									<SheetContent>
										<SheetHeader>
											<SheetTitle>Edit item</SheetTitle>
											<SheetDescription>
												Make changes to the item here. Click save when you're done.
											</SheetDescription>
										</SheetHeader>
										<Textarea v-model="modifyItemTextAreaValue" class="mb-4 mt-3 h-52" />
										<SheetFooter>
											<SheetClose as-child>
												<Button type="submit" @click="ModifyItem(element.id, group.groupName)">
													Save changes
												</Button>
											</SheetClose>
										</SheetFooter>
									</SheetContent>
								</Sheet>

								<button class="text-slate-600 transition-transform hover:scale-110" @click="DeleteItem(element.id, group.groupName)">
									<Trash2 :size="18" color="red" />
								</button>
							</div>

						</div>
					</template>
				</draggable>

				<button class="transition-bg-color mt-3 rounded-sm border border-slate-300 px-3 py-2 font-bold text-slate-700 shadow-lg hover:bg-slate-100" @click="(e) => AddItem(e, group.groupName)">
					+ ITEM
				</button>
			</div>

			<!-- Add new group button -->
			<Sheet>
				<SheetTrigger as-child>
					<button class="transition-bg-color min-w-80 rounded-lg border text-2xl font-bold text-slate-700 shadow-lg hover:bg-slate-100" @click="createGroupInputValue = 'New Group'">
						+ GROUP
					</button>
				</SheetTrigger>
				<SheetContent>
					<SheetHeader>
						<SheetTitle>Edit item</SheetTitle>
						<SheetDescription>
							Set a new group name here. Click save when you're done.
						</SheetDescription>
					</SheetHeader>
					<Input v-model="createGroupInputValue" class="mb-4 mt-3" />
					<SheetFooter>
						<SheetClose as-child>
							<Button type="submit" @click="AddGroup">
								Create group
							</Button>
						</SheetClose>
					</SheetFooter>
				</SheetContent>
			</Sheet>

		</div>
	</div>
</template>

<style scoped>
.transition-bg-color {
	transition-property: background-color;
	transition-duration: 0.15s;
	transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
}
</style>