import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { name, user } = req
  const { scopeName } = req.body

  // short-circuit fail-first
  if (!scopeName) {
    req.log('Scope Delete (Bad Request)', true)
    return req.respond({ status: 400, body: { error: 'Missing scopeName' } })
  } // end if
  const hasScopeAccess = user.permissions.scopes.find((s) => s.name === scopeName)
  const hasScopeEditAccess = hasScopeAccess && hasScopeAccess.value === 'edit'

  try {
    const targetScope = await req.broker.db.getItem(`scope:${scopeName}`)

    // short-circuit fail-first
    if (!hasScopeEditAccess) {
      req.log(`Scope Delete (${scopeName})`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${name}" does not have scope edit permission`
        }
      })
    } else if (!targetScope) {
      req.log(`Scope Delete (${scopeName}-NO_SCOPE)`, true)
      return req.respond({
        res,
        status: 401,
        body: {
          error: `Scope "${scopeName}" does not exist.`
        }
      })
    } else {
      req.log(`Scope Delete (${scopeName})`)
      const users = await req.broker.db.getItem('users')

      users.forEach((user) => {
        // remove any existance of the deleted scope throughout all users
        user.permissions.scopes = user.permissions.scopes.filter((s) => s.name !== scopeName)
      })

      await req.broker.db.setItem('users', users)
      await req.broker.db.removeItem(`scope:${scopeName}`)
      const scopes = await req.broker.db.getItem('scopes')

      await req.broker.db.setItem(
        'scopes',
        scopes.filter((s) => s.name !== scopeName)
      )
      req.respond({ body: { success: 'Deleted scope successfully.' } })
    } // end if
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error modifying user.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
