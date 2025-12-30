import express from 'express'
import chalk from 'chalk'
import { authenticate } from './authenticate.js'
import { sendMail } from '../libraries/sendMail.js'

const router = express.Router({ mergeParams: true })

router.post('/', express.text(), authenticate, async (req, res) => {
  const { user } = req

  try {
    const pendingWipe = await req.broker.db.getItem('pendingWipe')

    if (!pendingWipe) {
      req.log('Wipe Cancel (No Pending)', true)
      return req.respond({
        status: 400,
        body: {
          error: 'No pending wipe to cancel.'
        }
      })
    }

    // Check if wipe has already expired
    if (Date.now() >= pendingWipe.expiresAt) {
      req.log('Wipe Cancel (Already Expired)', true)
      return req.respond({
        status: 400,
        body: {
          error: 'Pending wipe has already expired and will execute shortly.'
        }
      })
    }

    // Cancel the wipe
    await req.broker.db.removeItem('pendingWipe')
    req.log(`Wipe Cancelled (by ${user.name})`)

    // Email all users about the cancellation
    const users = (await req.broker.db.getItem('users')) || []
    const userDetails = await Promise.all(users.map((u) => req.broker.db.getItem(`user:${u.name}`)))

    userDetails.forEach((u) => {
      if (u && u.email) {
        sendMail({
          broker: req.broker,
          to: u.email,
          title: 'Wipe Cancelled',
          body: `The pending wipe has been cancelled by "${user.name}".`
        })
      }
    })

    req.respond({
      body: {
        success: `Wipe cancelled successfully. The wipe was originally scheduled by "${pendingWipe.triggeredBy}".`
      }
    })
  } catch (err) {
    req.respond({ status: 500, body: { error: 'Server error cancelling wipe.' } })
    console.log(chalk.red(err))
  }
})

export default { router }
