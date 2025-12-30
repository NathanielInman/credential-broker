/**
 * Script to generate PGP test keys for unit tests
 * Run with: node tests/fixtures/generate-keys.js
 */
import * as openpgp from 'openpgp'
import { writeFileSync, mkdirSync, existsSync } from 'fs'
import { dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const keysDir = `${__dirname}/pgp-keys`

async function generateTestKeys() {
  console.log('Generating test PGP keys...')

  // Ensure directory exists
  if (!existsSync(keysDir)) {
    mkdirSync(keysDir, { recursive: true })
  }

  // Generate a new keypair
  const { privateKey, publicKey } = await openpgp.generateKey({
    type: 'ecc',
    curve: 'curve25519',
    userIDs: [{ name: 'Test User', email: 'test@example.com' }],
    passphrase: 'testpassword',
    format: 'armored'
  })

  // Write keys to files
  writeFileSync(`${keysDir}/test-private.asc`, privateKey)
  writeFileSync(`${keysDir}/test-public.asc`, publicKey)

  console.log('Keys generated successfully:')
  console.log(`  Private key: ${keysDir}/test-private.asc`)
  console.log(`  Public key: ${keysDir}/test-public.asc`)
  console.log('  Passphrase: testpassword')
}

generateTestKeys().catch(console.error)
