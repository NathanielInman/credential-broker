import { vi } from 'vitest'
import { createMockDb } from './database.js'

/**
 * Default broker strategies matching models/Broker.js
 */
const defaultStrategies = [
  { name: 'Oldest user acquires edit access', value: true },
  { name: 'Revoker acquires abandoned scope access', value: true },
  { name: 'All users allowed to revoke', value: true },
  { name: 'First account gets access', value: true },
  { name: 'Any user can cancel a wipe', value: true }
]

/**
 * Create a mock Express request object
 */
export function createMockReq(overrides = {}) {
  const db = overrides.broker?.db || createMockDb()

  return {
    ip: '127.0.0.1',
    headers: {},
    body: {},
    params: {},
    query: {},
    broker: {
      db,
      strategies: [...defaultStrategies],
      port: 4000,
      sessionTTL: 1000 * 60 * 5,
      twoFactorTTL: 1000 * 60 * 60 * 12,
      emailService: 'None',
      emailTransport: {},
      ...overrides.broker
    },
    log: vi.fn(),
    respond: vi.fn(),
    user: null,
    name: null,
    email: null,
    secret: null,
    key: null,
    ...overrides
  }
}

/**
 * Create a mock Express response object
 */
export function createMockRes() {
  const res = {
    status: vi.fn().mockReturnThis(),
    json: vi.fn().mockReturnThis(),
    send: vi.fn().mockReturnThis(),
    set: vi.fn().mockReturnThis(),
    end: vi.fn().mockReturnThis()
  }
  return res
}

/**
 * Create a mock next function for middleware
 */
export function createMockNext() {
  return vi.fn()
}

/**
 * Create a complete mock user object
 */
export function createMockUser(overrides = {}) {
  return {
    name: 'testuser',
    email: 'test@example.com',
    remoteIP: '127.0.0.1',
    lastAuthentication: new Date().toISOString(),
    lastAction: 'test',
    lastScope: '',
    usePassword: false,
    key: 'mock-public-key',
    permissions: {
      viewUsers: true,
      editUsers: true,
      viewScopeNames: true,
      createScopes: true,
      scopes: []
    },
    secret: null,
    ...overrides
  }
}

/**
 * Helper to simulate authenticated request
 */
export function createAuthenticatedReq(userOverrides = {}, reqOverrides = {}) {
  const user = createMockUser(userOverrides)
  return createMockReq({
    user,
    name: user.name,
    email: user.email,
    secret: 'mock-session-secret-32-chars-long!',
    ...reqOverrides
  })
}
