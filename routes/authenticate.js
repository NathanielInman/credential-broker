import speakeasy from 'speakeasy'
import { authSecureDecrypt } from '../libraries/authSecure.js'
import { verify } from '../libraries/verify.js'

export async function authenticate(req, res, next) {
  // short-circuit failure
  if (!req.headers.id) {
    req.log(`${req.originalUrl}: Key missing`, true)
    return res.status(401).send('User key was not passed in the header.')
  } // end if

  // short-circuit failure
  if (!req.headers.name) {
    req.log(`${req.originalUrl}: Username missing`, true)
    return res.status(401).send('User name was not passed in the header.')
  } // end if

  // ensure there's a valid active and identified session to help secure
  // traffic back-and-forth.
  try {
    const session = await req.broker.db.getItem(`session:${req.headers.id}`)

    // we omit logging this, it's an expected behavior to have to re-auth
    if (!session) return res.status(401).send('Session expired.')
    const { secret, authenticated } = session
    const name = authSecureDecrypt(secret, req.headers.name)
    const email = authSecureDecrypt(secret, req.headers.email)

    req.secret = secret // diffie-hellman key to encrypt transmission payloads
    req.name = name
    req.email = email
    if (!authenticated) return res.status(401).send('Session expired.')
  } catch (err) {
    console.log(err)
    req.log(`${req.originalUrl}: Authentication Failure`, true)
    return res.status(401).send('Improper authentication headers.')
  }

  const user = await req.broker.db.getItem(`user:${req.name}`)

  // short-circuit failure
  if (!user) {
    req.log(`${req.originalUrl}: User does not exist`, true)
    return res.status(401).send('User does not exist.')
  } else {
    // attach the user to the request so any route can attach their
    // individual requirements or filters based on their permissions
    req.user = user
    req.key = user.key // public key used to encrypt payload
  } // end if

  // ensure there's a validated two-factor token. The two-factor TTL can
  // be configured during broker server setup.
  try {
    const twoFactorSession = await req.broker.db.getItem(`twofactor:${req.headers.id}`)

    // we omit logging this, it's an expected behavior to have to re-auth
    if (!twoFactorSession && !req.headers['two-factor-token']) {
      return res.status(401).send('Two-factor expired.')
    } else if (!twoFactorSession && req.headers['two-factor-token']) {
      const valid = speakeasy.totp.verify({
        secret: user.twoFactorSecret,
        encoding: 'base32',
        token: req.headers['two-factor-token']
      })

      if (!valid) {
        req.log(`${req.originalUrl}: Invalid two-factor token`, true)
        return res.status(401).send('Invalid Two-factor token.')
      } else {
        req.broker.db.setItem(`twofactor:${req.headers.id}`, true, { ttl: req.broker.twoFactorTTL })
      } // end if
    } // end if
  } catch (err) {
    console.log(err)
    req.log(`${req.originalURL}: Two-Factor Authentication Failure`, true)
    return res.status(401).send('Improper authentication headers.')
  }

  // short-circuit failure
  if (!user.permissions) {
    req.log(`${req.originalUrl}: User data is corrupted`, true)
    return req.respond({
      status: 401,
      body: { error: 'Your user structure invalid, data corrupted.' }
    })
  } // end if

  // a body was sent with the request, verify the identity of the body
  // matches the users public key; otherwise the request was tampered with
  // or someone is spoofing or impersonating the user
  if (req.body) {
    try {
      req.body = await verify(user.key, req.body)
    } catch (err) {
      console.log(err)
      req.log(`${req.originalUrl}: Signing verification failure`, true)
      return req.respond({
        status: 401,
        body: { error: 'Request has been tampered with!' }
      })
    }
  } // end if
  next()
}

export default { authenticate }
