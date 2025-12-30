import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'
import { sendMail } from '../libraries/sendMail.js'

const router = express.Router({ mergeParams: true })
const WIPE_DELAY_MS = 24 * 60 * 60 * 1000 // 24 hours

router.post('/', express.text(), authenticate, async (req, res) => {
  const { user } = req

  try {
    // Check if there's already a pending wipe
    const existingWipe = await req.broker.db.getItem('pendingWipe')
    if (existingWipe) {
      req.log('Wipe (Already Pending)', true)
      return req.respond({
        status: 400,
        body: {
          error: 'A wipe is already pending. Cancel it first or wait for it to execute.'
        }
      })
    }

    // Check if "Any user can cancel a wipe" strategy is enabled (index 4)
    const canCancelWipe = req.broker.strategies[4].value

    if (canCancelWipe) {
      // Schedule wipe with 24-hour delay
      const pendingWipe = {
        triggeredAt: Date.now(),
        triggeredBy: user.name,
        expiresAt: Date.now() + WIPE_DELAY_MS
      }

      await req.broker.db.setItem('pendingWipe', pendingWipe)
      req.log(`Wipe Scheduled (by ${user.name})`)

      // Email all users about the pending wipe
      const users = (await req.broker.db.getItem('users')) || []
      const userDetails = await Promise.all(
        users.map((u) => req.broker.db.getItem(`user:${u.name}`))
      )

      userDetails.forEach((u) => {
        if (u && u.email) {
          sendMail({
            broker: req.broker,
            to: u.email,
            title: 'Wipe Scheduled',
            body: `A wipe has been scheduled by "${user.name}". All users will be removed in 24 hours. Run "broker wipe cancel" to stop this.`
          })
        }
      })

      req.respond({
        body: {
          success:
            'Wipe scheduled. All users will be removed in 24 hours. Users have been notified via email.'
        }
      })
    } else {
      // Execute wipe immediately
      req.log(`Wipe Executed (by ${user.name})`)
      const users = (await req.broker.db.getItem('users')) || []

      for (const u of users) {
        await req.broker.db.removeItem(`user:${u.name}`)
      }
      await req.broker.db.setItem('users', [])

      req.respond({
        body: {
          success: 'Wipe executed. All users have been removed.'
        }
      })
    }
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error executing wipe.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
