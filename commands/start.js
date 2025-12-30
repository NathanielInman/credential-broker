import fs from 'fs'
import chalk from 'chalk'
import express from 'express'
import { Broker } from '../models/Broker.js'
import { log } from '../libraries/log.js'
import { respond } from '../libraries/respond.js'
import routes from '../routes/index.js'

export async function start() {
  let broker

  if (!fs.existsSync('./broker.json')) {
    broker = new Broker()

    await broker.askStrategies()
    fs.writeFileSync('./broker.json', JSON.stringify(broker))
  } else {
    broker = new Broker(JSON.parse(fs.readFileSync('./broker.json')))
  } // end if
  const app = express()

  app.use(async (req, res, next) => {
    req.ip = req.headers['x-forwarded-for'] || req.connection.remoteAddress
    req.key = decodeURIComponent(req.headers.key || '')
    req.log = (string, isError) => log(req.ip, req.name, string, isError)
    req.respond = ({ status = 200, body = '' }) => respond({ req, res, status, body })

    // ensure the broker settings are loaded, they can be used for configuration
    await broker.dbLoading
    req.broker = broker
    next()
  })
  app.use(routes.router)
  console.log(chalk.blue('         ./*.           '))
  console.log(chalk.blue('     /@@@@@@@@@@.       '))
  console.log(chalk.blue('   ,@@@@,    /@@@@      '))
  console.log(chalk.blue('  .@@@.        *@@@     '))
  console.log(chalk.blue('  @@@,          &@@%    '))
  console.log(chalk.blue('  @@@.          %@@%    '))
  console.log(chalk.blue('  @@@.          %@@%    '))
  console.log(chalk.blue('@@@@@@@@@@@@@@@@@@@@@@, ') + broker.getVersionNumber())
  console.log(chalk.blue('@@@@@@@@@@%%@@@@@@@@@@, ') + (await broker.getAddress()))
  console.log(chalk.blue('@@@@@@@@#    ,@@@@@@@@, ') + broker.getStrategyString(0))
  console.log(chalk.blue('@@@@@@@@&    #@@@@@@@@, ') + broker.getStrategyString(1))
  console.log(chalk.blue('@@@@@@@@@.   @@@@@@@@@, ') + broker.getStrategyString(2))
  console.log(chalk.blue('@@@@@@@@@@#(@@@@@@@@@@, ') + broker.getStrategyString(3))
  console.log(chalk.blue('@@@@@@@@@@@@@@@@@@@@@@  ') + broker.getStrategyString(4))
  console.log(broker.getSessionTTL())
  console.log(broker.getTwoFactorTTL())
  console.log(broker.getServerEmail())
  app.listen(broker.port)

  // Check for pending wipes every minute
  setInterval(async () => {
    const pendingWipe = await broker.db.getItem('pendingWipe')
    if (pendingWipe && Date.now() >= pendingWipe.expiresAt) {
      // Execute wipe: delete all users
      const users = (await broker.db.getItem('users')) || []
      for (const user of users) {
        await broker.db.removeItem(`user:${user.name}`)
      }
      await broker.db.setItem('users', [])
      await broker.db.removeItem('pendingWipe')
      console.log(chalk.yellow('Pending wipe executed - all users removed'))
    }
  }, 60000) // Check every minute
}
