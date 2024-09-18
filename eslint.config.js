import hagemanto from "eslint-plugin-hagemanto";
import tailwind from "eslint-plugin-tailwindcss";
import vue from "eslint-plugin-vue";
import globals from "globals";

export default [
    { files: ["src/**/*.{vue,ts}"] },
    { ignores: ["src/components/ui/*"] },

    ...hagemanto(),
    ...tailwind.configs["flat/recommended"],
    ...vue.configs["flat/recommended"],

    {
        rules: {
            "tailwindcss/no-custom-classname": "off",
            "vue/html-indent": ["error", "tab"],
            "vue/multi-word-component-names": "off",
            "vue/no-unused-vars": ["error", {
                "ignorePattern": "^_"
            }]
        }
    },
    {
        languageOptions: {
            globals: globals.browser, parserOptions: {
                project: true, parser: "@typescript-eslint/parser", extraFileExtensions: [".vue"]
            }
        }
    },
];