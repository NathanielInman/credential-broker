/**
 * Mock user data for tests
 */

export const testUser = {
  name: 'testuser',
  email: 'test@example.com',
  remoteIP: '127.0.0.1',
  lastAuthentication: '2024-01-01T00:00:00.000Z',
  lastAction: 'test',
  lastScope: '',
  usePassword: false,
  key: null, // Set from test-public.asc in tests
  permissions: {
    viewUsers: true,
    editUsers: true,
    viewScopeNames: true,
    createScopes: true,
    scopes: [{ name: 'test-scope', value: 'edit' }]
  },
  secret: null
}

export const adminUser = {
  ...testUser,
  name: 'admin',
  email: 'admin@example.com',
  permissions: {
    viewUsers: true,
    editUsers: true,
    viewScopeNames: true,
    createScopes: true,
    scopes: []
  }
}

export const viewOnlyUser = {
  ...testUser,
  name: 'viewer',
  email: 'viewer@example.com',
  permissions: {
    viewUsers: true,
    editUsers: false,
    viewScopeNames: true,
    createScopes: false,
    scopes: [{ name: 'test-scope', value: 'view' }]
  }
}

export const restrictedUser = {
  ...testUser,
  name: 'restricted',
  email: 'restricted@example.com',
  permissions: {
    viewUsers: false,
    editUsers: false,
    viewScopeNames: false,
    createScopes: false,
    scopes: []
  }
}

// Password for test PGP keys
export const testPassword = 'testpassword'
