import express from 'express'
import { log } from '../libraries/log.js'
import { authSecureEncrypt, authSecureDecrypt } from '../libraries/authSecure.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.json(), async (req, res) => {
  const secret = await req.broker.db.getItem(`session:${req.headers.key}`)
  const name = authSecureDecrypt(secret, req.headers.name)
  const email = authSecureDecrypt(secret, req.headers.email)
  const { ip } = req
  const { key } = req.body
  const user = await req.broker.db.getItem(`user:${name}`)

  // short-circuit success-first
  if (user) {
    // override the existing users public key in case the user changed it
    log(ip, name, 'Initialize User')
    if (user.key !== req.body.key) {
      user.key = req.body.key
      await req.broker.db.setItem(`user:${name}`, user)
      return res.status(200).send(
        authSecureEncrypt(
          secret,
          JSON.stringify({
            success: 'Your user is properly linked. Updated public key.'
          })
        )
      )
    } else {
      return res.status(200).send(
        authSecureEncrypt(
          secret,
          JSON.stringify({
            success: 'Your user is properly linked'
          })
        )
      )
    } // end if
  } // end if
  const users = (await req.broker.db.getItem('users')) || []
  const scopes = (await req.broker.db.getItem('scopes')) || []

  if (!scopes && !users.length) {
    const newUser = {
      date: new Date().toISOString(),
      name,
      email,
      key,
      addedBy: 'ROOT',
      addedByIP: ip,
      permissions: {
        viewUsers: req.body.permissions.viewUsers,
        editUsers: req.body.permissions.editUsers,
        viewScopeNames: req.body.permissions.viewScopeNames,
        createScopes: req.body.permissions.createScopes,
        scopes: req.body.permissions.scopes.map((s) => ({ name: s.name, value: s.value }))
      }
    }

    log(ip, name, 'Initialize User (FIRST)')
    await req.broker.db.setItem(`user:${name}`, newUser)
    await req.broker.db.setItem('users', [{ name: newUser.name }])
    res.status(200).send(
      authSecureEncrypt(
        secret,
        JSON.stringify({
          success: 'First user created successfully with requested permissions.'
        })
      )
    )
  } else if (!users.length) {
    const { value } = req.broker.strategies.find((s) => s.name === 'First account gets access')

    if (value) {
      log(ip, name, 'Initialize User (FIRST-GAINED-ALL-ACCESS)')
      const newUser = {
        date: new Date().toISOString(),
        name,
        email,
        key,
        addedBy: 'ROOT',
        addedByIP: ip,
        permissions: {
          viewUsers: req.body.permissions.viewUsers,
          editUsers: req.body.permissions.editUsers,
          viewScopeNames: req.body.permissions.viewScopeNames,
          createScopes: req.body.permissions.createScopes,
          scopes: scopes.map((scope) => ({ name: scope.name, value: false }))
        }
      }

      await req.broker.db.setItem(`user:${name}`, newUser)
      await req.broker.db.setItem('users', [{ name: newUser.name }])
      res.status(200).send(
        authSecureEncrypt(
          secret,
          JSON.stringify({
            success: `First user created successfully and given access to scopes: ${scopes.map((s) => s.name).join()}.`
          })
        )
      )
    } else {
      const newUser = {
        date: new Date().toISOString(),
        name,
        email,
        key,
        addedBy: 'ROOT',
        addedByIP: ip,
        permissions: {
          viewUsers: false,
          editUsers: false,
          viewScopeNames: false,
          createScopes: false,
          scopes: req.body.permissions.scopes.map((s) => ({ name: s.name, value: false }))
        }
      }

      await req.broker.db.setItem(`user:${name}`, newUser)
      await req.broker.db.setItem('users', [{ name: newUser.name }])
      log(ip, name, 'Initialize User (FIRST-ALL-ACCESS-DENIED)', true)
      res.status(200).send(
        authSecureEncrypt(
          secret,
          JSON.stringify({
            success:
              'User created but without any permissions because "First account strategy get access" strategy turned off.'
          })
        )
      )
    } // end if
  } else {
    const userList = await Promise.all(
      users.map((user) => req.broker.db.getItem(`user:${user.name}`))
    )
    const allowedUsers = userList
      .filter((user) => user.permissions.editUsers)
      .map((user) => user.name)

    log(ip, name, 'Initialize User (NOT-SETUP)', true)
    res.status(400).send(
      authSecureEncrypt(
        secret,
        JSON.stringify({
          error: `In order to be added as a user, please contact user administrators: ${allowedUsers.join()}`
        })
      )
    )
  } // end if
})

export default { router }
