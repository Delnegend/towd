// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
	ssr: false,
	devtools: { enabled: false },
	srcDir: "src",
	css: ["~/assets/main.css"],
	modules: [
		"@nuxtjs/tailwindcss",
		"shadcn-nuxt",
		"@nuxtjs/color-mode",
		"@nuxt/eslint",
	],
	shadcn: {
		/**
		 * Prefix for all the imported component
		 */
		prefix: "",

		/**
		 * Directory that the component lives in.
		 * @default "./components/ui"
		 */
		componentDir: "./src/components/ui"
	},
	},
	compatibilityDate: "2024-08-28",
});
