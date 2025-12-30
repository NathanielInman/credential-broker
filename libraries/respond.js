import { encrypt } from './authPGP.js'
import { authSecureEncrypt } from './authSecure.js'

export async function respond({ req, res, status = 200, body = '' } = {}) {
  const { key, secret } = req
  const pgpEncrypted = await encrypt(key, JSON.stringify(body))
  const sessionEncrypted = authSecureEncrypt(secret, pgpEncrypted)

  return res.status(status).send(sessionEncrypted)
}
