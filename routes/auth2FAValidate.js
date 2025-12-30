import express from 'express'
import speakeasy from 'speakeasy'
import { authSecureDecrypt } from '../libraries/authSecure.js'
import { verify } from '../libraries/verify.js'
import { log } from '../libraries/log.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), async (req, res) => {
  // short-circuit failure
  if (!req.headers.id) {
    req.log('auth2FAValidate: Id missing', true)
    return res.status(401).send('User session id was not passed in the header.')
  } else if (!req.headers.name) {
    req.log('auth2FAValidate: Name missing', true)
    return res.status(401).send('Encrypted name missing from the header.')
  } // end if
  const session = await req.broker.db.getItem(`session:${req.headers.id}`)

  if (!session) {
    req.log('auth2FAValidate: No session to attach', true)
    return res.status(401).send('Session expired.')
  } // end if
  const { secret } = session
  const name = authSecureDecrypt(secret, req.headers.name)
  const user = await req.broker.db.getItem(`user:${name}`)
  const token = await verify(user.key, req.body)
  const valid = speakeasy.totp.verify({
    secret: user.twoFactorSecret,
    encoding: 'base32',
    token
  })

  if (valid) {
    log(req.ip, user.name, 'auth2FAValidate: Successfully setup 2FA')
    req.broker.db.setItem(`user:${name}`, { ...user, twoFactorActive: true })
  } // end if
  req.key = user.key
  req.secret = secret
  req.respond({ body: { valid } })
})

export default { router }
