import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { name, user } = req

  try {
    // short-circuit fail-first
    if (!user.permissions.viewUsers) {
      req.log('Get All Users', true)
      return req.respond({
        status: 401,
        body: {
          error: `User "${name}" does not have user edit permission.`
        }
      })
    } else {
      req.log('Get All Users')
      const users = await req.broker.db.getItem('users')
      const userData = (users || []).map((user) => user.name)

      req.respond({ body: { success: userData } })
    } // end if
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error retrieving users.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
