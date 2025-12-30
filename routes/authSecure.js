import express from 'express'
import crypto from 'crypto'
import { log } from '../libraries/log.js'

const router = express.Router({ mergeParams: true })

router.post('/', async (req, res) => {
  // short-circuit failure
  if (!req.headers.key) {
    req.log('authSecure: Key missing', true)
    return res.status(401).send('User key was not passed in the header.')
  } else if (!req.headers.id) {
    req.log('authSecure: Id missing', true)
    return res.status(401).send('User session id was not passed in the header.')
  } else if (!req.headers.prime) {
    req.log('authSecure: Prime missing', true)
    return res.status(401).send('Diffie hellman prime missing in the header.')
  } // end if

  const { id, key, prime } = req.headers

  const server = crypto.createDiffieHellman(prime, 'base64')
  const serverKey = server.generateKeys('base64')
  const secret = server.computeSecret(key, 'base64', 'base64')
  const ip = req.headers['x-forwarded-for'] || req.connection.remoteAddress

  log(ip, id, 'Auth Secure')
  await req.broker.db.setItem(
    `session:${id}`,
    { secret, authenticated: false },
    { ttl: req.broker.sessionTTL }
  )
  res.status(200).json(serverKey)
})

export default { router }
