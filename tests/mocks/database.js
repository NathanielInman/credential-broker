/**
 * In-memory mock for node-persist database
 */
export function createMockDb() {
  const data = new Map()

  return {
    async getItem(key) {
      return data.get(key)
    },

    async setItem(key, value, options = {}) {
      data.set(key, value)
      return value
    },

    async removeItem(key) {
      data.delete(key)
    },

    async keys() {
      return Array.from(data.keys())
    },

    async values() {
      return Array.from(data.values())
    },

    async length() {
      return data.size
    },

    async clear() {
      data.clear()
    },

    // Helper for tests to inspect state
    _getData() {
      return data
    },

    // Helper to seed test data
    _seed(entries) {
      for (const [key, value] of Object.entries(entries)) {
        data.set(key, value)
      }
    }
  }
}
