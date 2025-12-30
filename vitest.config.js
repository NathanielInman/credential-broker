import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['libraries/**', 'routes/**'],
      exclude: ['**/node_modules/**', '**/tests/**']
    },
    include: ['tests/**/*.test.js']
  }
})
