/**
 * Mock broker configuration for tests
 */

export const defaultStrategies = [
  { name: 'Oldest user acquires edit access', value: true },
  { name: 'Revoker acquires abandoned scope access', value: true },
  { name: 'All users allowed to revoke', value: true },
  { name: 'First account gets access', value: true },
  { name: 'Any user can cancel a wipe', value: true }
]

export const testBrokerConfig = {
  externalIP: '127.0.0.1',
  port: 4000,
  scopeNumber: 0,
  userNumber: 0,
  lastAccess: '',
  lastUser: '',
  strategies: [...defaultStrategies],
  emailService: 'None',
  emailTransport: {},
  sessionTTL: 1000 * 60 * 5, // 5 minutes
  twoFactorTTL: 1000 * 60 * 60 * 12 // 12 hours
}

export const brokerWithEmail = {
  ...testBrokerConfig,
  emailService: 'SMTP',
  emailTransport: {
    host: 'smtp.test.com',
    port: 587,
    tls: true,
    auth: {
      type: 'login',
      user: 'test@test.com',
      pass: 'testpass'
    }
  }
}

// Broker with wipe cancellation disabled
export const brokerNoWipeCancel = {
  ...testBrokerConfig,
  strategies: [
    ...defaultStrategies.slice(0, 4),
    { name: 'Any user can cancel a wipe', value: false }
  ]
}
