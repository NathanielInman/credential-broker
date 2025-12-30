import { vi } from 'vitest'

/**
 * Create a mock fetch function with customizable responses
 */
export function createMockFetch(responses = {}) {
  const defaultResponses = {
    '/authSecure': {
      ok: true,
      json: async () => ({ key: 'mock-server-dh-key' })
    },
    '/authIdentify': {
      ok: true,
      text: async () => 'encrypted-challenge'
    },
    '/authChallenge': {
      ok: true,
      text: async () => JSON.stringify({ success: true })
    },
    ...responses
  }

  return vi.fn(async (url, options = {}) => {
    // Extract path from URL
    const path = typeof url === 'string' ? new URL(url, 'http://localhost').pathname : url.pathname

    const response = defaultResponses[path] || {
      ok: true,
      json: async () => ({}),
      text: async () => ''
    }

    return {
      ok: response.ok ?? true,
      status: response.status ?? 200,
      json: response.json || (async () => ({})),
      text: response.text || (async () => ''),
      headers: new Map(Object.entries(response.headers || {}))
    }
  })
}

/**
 * Create a mock fetch that always fails
 */
export function createFailingFetch(error = new Error('Network error')) {
  return vi.fn(async () => {
    throw error
  })
}

/**
 * Create a mock fetch with sequential responses
 */
export function createSequentialFetch(responses) {
  let index = 0
  return vi.fn(async () => {
    const response = responses[index] || responses[responses.length - 1]
    index++
    return {
      ok: response.ok ?? true,
      status: response.status ?? 200,
      json: async () => response.json || {},
      text: async () => response.text || ''
    }
  })
}
