<script setup lang="ts">
import { Label } from "radix-vue";
import { toast } from "vue-sonner";
import { Login } from "~/lib/api";

const secretKey = ref("");

async function Verify() {
	try {
		await Login(secretKey.value);
		toast.success("Successfully verified secret key");
	} catch (error) {
		toast.error(`${error}`);
	}
}
</script>

<template>
	<div class="mx-auto flex min-h-[calc(100vh-64px)] max-w-xs flex-col justify-center gap-y-2">
		<Label for="terms">Secret Key</Label>
		<Input v-model="secretKey" type="password" placeholder="ddefc197-62fa-4c09-be7b-0c63bf7ef36a" />
		<Button class="mt-2" @click="Verify">Verify</Button>
	</div>
</template>