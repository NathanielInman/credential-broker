import fs from 'fs'
import crypto from 'crypto'
import fetch from 'node-fetch'

export async function authSecure(user) {
  const diffieHellman = crypto.createDiffieHellman(512)
  const prime = diffieHellman.getPrime('base64')
  const client = crypto.createDiffieHellman(prime, 'base64')
  const key = client.generateKeys('base64')
  const id = crypto.createHash('md5').update(user.name).digest('hex')

  const serverKey = await fetch(`${user.remoteIP}/authSecure`, {
    method: 'POST',
    headers: { id, key, prime }
  }).then((res) => res.json())

  const secret = client.computeSecret(serverKey, 'base64', 'base64')

  user.secret = secret
  fs.writeFileSync('./user.json', JSON.stringify(user))
}

export function authSecureEncrypt(secret, content) {
  const iv = crypto.randomBytes(16)
  const cipher = crypto.createCipheriv('aes-256-cbc', secret.substr(0, 32), iv)
  const encrypted = cipher.update(content, 'utf8', 'base64') + cipher.final('base64')

  return encodeURIComponent(`${iv.toString('hex')}:${encrypted}`)
}

export function authSecureDecrypt(secret, content) {
  const parts = decodeURIComponent(content).split(':')
  const iv = Buffer.from(parts.shift(), 'hex')
  const decipher = crypto.createDecipheriv('aes-256-cbc', secret.substr(0, 32), iv)
  const decrypted = decipher.update(parts.shift(), 'base64', 'utf8') + decipher.final('utf8')

  return decrypted
}
