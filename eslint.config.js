import hagemanto from "eslint-plugin-hagemanto";
import tailwind from "eslint-plugin-tailwindcss";
import globals from "globals";
import withNuxt from './.nuxt/eslint.config.mjs';

export default withNuxt([
	{
		name: "towd/specific",
		rules: {
			"tailwindcss/no-custom-classname": "off",
		}
	},
]).prepend([
	{ name: "towd/include-exclude", files: ["src/**/*.{vue,ts}"], ignores: ["src/components/ui/*"] },
	...hagemanto(),
	...tailwind.configs["flat/recommended"],

	{
		name: "towd/language-options",
		languageOptions: {
			globals: globals.browser, parserOptions: {
				project: "./tsconfig.json", parser: "@typescript-eslint/parser", extraFileExtensions: [".vue"]
			}
		}
	},
]);