import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { ip, name, user } = req
  const requestedUsername = req.body

  if (!requestedUsername) {
    req.log('User Modify (Bad Request)', true)
    return req.respond({
      status: 400,
      body: {
        error: 'Missing requestedUsername'
      }
    })
  } // end if
  try {
    const targetUser = await req.broker.db.getItem(`user:${requestedUsername}`)

    // short-circuit fail-first
    if (!user.permissions.editUsers) {
      req.log(`User Modify (${req.body.name})`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${name}" does not have user edit permission.`
        }
      })
    } else if (!targetUser) {
      req.log(`User Modify (${req.body.name}-NO-USER)`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${name}" does not exist.`
        }
      })
    } else {
      req.log(`User Modify (${req.body.name})`)
      await req.broker.db.setItem(`user:${req.body.name}`, {
        date: new Date().toISOString(),
        name: req.body.name,
        email: req.body.email,
        addedBy: name,
        addedByIP: ip,
        key: req.body.key,
        permissions: {
          viewUsers: req.body.permissions.viewUsers,
          editUsers: req.body.permissions.editUsers,
          viewScopeNames: req.body.permissions.viewScopeNames,
          createScopes: req.body.permissions.createScopes,
          scopes: req.body.permissions.scopes.map((s) => ({ name: s.name, value: s.value }))
        }
      })
      req.respond({ body: { success: 'Modified user successfully.' } })
    } // end if
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error modifying user.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
