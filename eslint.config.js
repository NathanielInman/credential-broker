import neostandard from 'neostandard'
import eslintConfigPrettier from 'eslint-config-prettier'

export default [
  ...neostandard(),
  eslintConfigPrettier,
  {
    ignores: ['node_modules/**', 'dist/**', 'coverage/**', 'data/**', 'tests/fixtures/pgp-keys/**']
  },
  {
    rules: {
      // Allow console for CLI application
      'no-console': 'off'
    }
  }
]
