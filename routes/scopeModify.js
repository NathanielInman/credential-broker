import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { name, user } = req
  const { target, scopeName, scopePublicKey } = req.body

  if (!target || !scopeName || !scopePublicKey) {
    req.log('Scope Modify (Bad Request)', true)
    return req.respond({
      status: 400,
      body: {
        error: 'Missing target, scopeName or scopePublicKey'
      }
    })
  } // end if
  const hasScopeAccess = user.permissions.scopes.find((s) => s.name === scopeName)
  const hasScopeEditAccess = hasScopeAccess && hasScopeAccess.value === 'edit'

  try {
    const targetScope = await req.broker.db.getItem(`scope:${target}`)

    // short-circuit fail-first
    if (!hasScopeEditAccess) {
      req.log(`Scope Modify (${scopeName})`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${name}" does not have scope edit permission.`
        }
      })
    } else if (!targetScope) {
      req.log(`Scope Modify (${scopeName}-NO-SCOPE)`, true)
      return req.respond({
        status: 401,
        body: {
          error: `Scope "${scopeName}" does not exist.`
        }
      })
    } else {
      req.log(`Scope Modify (${target}->${scopeName})`)
      targetScope.name = scopeName
      targetScope.scopePublicKey = scopePublicKey
      if (target !== scopeName) {
        await req.broker.db.removeItem(`scope:${target}`)
        await req.broker.db.setItem(`scope:${scopeName}`)
        const users = await req.broker.db.getItem('users')

        users.forEach(async (user) => {
          // alter any existing users access to the original scopename
          const scope = user.permissions.scopes.find((s) => s.name === target)

          // for any user that had access to the scope, change the scopename
          // to the new name and update that users scope name
          if (scope) {
            scope.name = scopeName
            await req.broker.db.setItem(`user:${name}`, user)
          } // end if
        })
      } else {
        // just public key updated
        await req.broker.db.setItem(`scope:${scopeName}`)
      } // end if
      req.respond({ body: { success: 'Modified scope successfully.' } })
    } // end if
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error modifying user.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
