import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';
export default tseslint.config({ignores:['dist','node_modules','components/ui']}, ...tseslint.configs.recommended, {
  files: ['web/**/*.{ts,tsx}'], plugins: {'react-hooks':reactHooks},
  rules: {...reactHooks.configs.recommended.rules, '@typescript-eslint/no-unused-vars':['error',{argsIgnorePattern:'^_'}]},
});
