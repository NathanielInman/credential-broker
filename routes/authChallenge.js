import express from 'express'
import crypto from 'crypto'
import { authSecureDecrypt } from '../libraries/authSecure.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), async (req, res) => {
  // short-circuit failure
  if (!req.headers.id) {
    req.log('authChallenge: Id missing', true)
    return res.status(401).send('User session id was not passed in the header.')
  } else if (!req.headers.name) {
    req.log('authChallenge: Name missing', true)
    return res.status(401).send('Encrypted name missing from the header.')
  } else if (!req.headers.challenge) {
    req.log('authChallenge: Challenge missing', true)
    return res.status(401).send('Encrypted challenge missing from the header.')
  } // end if
  const session = await req.broker.db.getItem(`session:${req.headers.id}`)

  if (!session) {
    req.log('authChallenge: No session to attach', true)
    return res.status(401).send('Session expired.')
  } // end if
  const { secret, challenge } = session
  const name = authSecureDecrypt(secret, req.headers.name)
  const challengeResponse = authSecureDecrypt(secret, req.headers.challenge)
  const user = await req.broker.db.getItem(`user:${name}`)
  const challengeExpected = crypto.createHash('md5').update(challenge).digest('hex')

  if (challengeResponse !== challengeExpected) {
    req.log('authChallenge: Failure to match', true)
    return res.status(401).send('Challenge does not match')
  } // end if
  req.broker.db.setItem(
    `session:${req.headers.id}`,
    { secret, authenticated: true },
    { ttl: req.broker.sessionTTL }
  )
  req.key = user.key
  req.secret = secret
  req.respond({ body: 'Authenticated' })
})

export default { router }
