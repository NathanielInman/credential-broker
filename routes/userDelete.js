import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { user } = req
  const { target } = req.body

  if (!target) {
    req.log('User Delete (Bad Request)', true)
    return req.respond({
      status: 400,
      body: {
        error: 'Request has been tempered with!'
      }
    })
  } // end if
  try {
    const targetUser = await req.broker.db.getItem(`user:${target}`)

    // short-circuit fail-first
    if (!user.permissions.editUsers) {
      req.log(`User Delete (${target})`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${target}" does not have user edit permission.`
        }
      })
    } else if (!targetUser) {
      req.log(`User Delete (${target}-NO-USER)`, true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${target}" does not exist.`
        }
      })
    } else {
      req.log(`User Delete (${target})`)
      const users = await req.broker.db.getItem('users')

      await req.broker.db.setItem(
        'users',
        users.filter((u) => u.name !== target)
      )
      await req.broker.db.removeItem(`user:${target}`)
      req.respond({ body: { success: 'Deleted user successfully.' } })
    } // end if
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error modifying user.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
