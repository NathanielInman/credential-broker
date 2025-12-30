import chalk from 'chalk'
import fs from 'fs'
import fetch from 'node-fetch'
import readline from 'readline'
import storage from 'node-persist'
import nodemailer from 'nodemailer'
import { spinner } from '../libraries/spinner.js'
import { sleep } from '../libraries/sleep.js'
import { sendMail } from '../libraries/sendMail.js'
import { password, prompt, confirm } from '../libraries/prompt.js'

const defaultSessionTTL = 1000 * 60 * 5 // 5 minutes
const defaultTwoFactorTTL = 1000 * 60 * 60 * 12 // 12 hours
const defaultStrategies = [
  { name: 'Oldest user acquires edit access', value: true },
  { name: 'Revoker acquires abandoned scope access', value: true },
  { name: 'All users allowed to revoke', value: true },
  { name: 'First account gets access', value: true },
  { name: 'Any user can cancel a wipe', value: true }
]

function prettyPrintMS(ms) {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  const weeks = Math.floor(days / 7)

  const result = []

  if (weeks) result.push(`${weeks} week${weeks > 1 ? 's' : ''}`)
  if (days) result.push(`${days % 7} day${days > 1 ? 's' : ''}`)
  if (hours) result.push(`${hours % 24} hour${hours > 1 ? 's' : ''}`)
  if (minutes) result.push(`${minutes % 60} minute${minutes > 1 ? 's' : ''}`)
  if (seconds) result.push(`${seconds % 60} second${seconds > 1 ? 's' : ''}`)
  result.push(`${ms % 1000} millisecond${ms > 1 ? 's' : ''}`)
  return result.join(' ')
}

export class Broker {
  constructor({
    externalIP = '',
    port = 4000,
    scopeNumber = 0,
    userNumber = 0,
    lastAccess = '',
    lastUser = '',
    strategies = defaultStrategies,
    emailService = 'None',
    emailTransport = {},
    sessionTTL = defaultSessionTTL,
    twoFactorTTL = defaultTwoFactorTTL
  } = {}) {
    this.externalIP = externalIP
    this.port = port
    this.scopeNumber = scopeNumber
    this.userNumber = userNumber
    this.lastAccess = lastAccess
    this.lastUser = lastUser
    this.strategies = strategies
    this.sessionTTL = sessionTTL
    this.twoFactorTTL = twoFactorTTL
    this.emailService = emailService
    this.emailTransport = emailTransport
    this.db = storage
    this.dbLoading = storage.init({
      dir: './data/',
      logging: false,
      ttl: false
    })
  }

  async askStrategies() {
    let answer

    answer = await confirm(chalk.green('Use email service?'))
    if (answer) {
      this.emailService = 'SMTP'
      do {
        this.emailTransport.host = await prompt(chalk.green('Enter email host: '))
        answer = await confirm(chalk.green(`Is this correct: "${this.emailTransport.host}"`))
        if (!answer) console.log("No problem, let's try again.")
      } while (!answer)
      do {
        this.emailTransport.port = parseInt(await prompt(chalk.green('Enter email port: ')))
        answer = await confirm(chalk.green(`Is this correct: "${this.emailTransport.port}"`))
        if (!answer) console.log("No problem, let's try again")
      } while (!answer)
      this.emailTransport.tls = await confirm(chalk.green('Use TLS with email'))
      this.emailTransport.auth = { type: 'login', user: '', pass: '' }
      do {
        this.emailTransport.auth.user = await prompt(chalk.green('Enter email username: '))
        answer = await confirm(chalk.green(`Is this correct: "${this.emailTransport.auth.user}"`))
        if (!answer) console.log("No problem, let's try again")
      } while (!answer)
      do {
        this.emailTransport.auth.pass = await password(chalk.green('Enter email password: '))
        const confirmPass = await password(chalk.green('Please confirm password: '))

        if (this.emailTransport.auth.pass !== confirmPass) {
          answer = false
          console.log("Passwords do not match, let's try again")
        }
      } while (!answer)
      if (await confirm(chalk.green('Use OAuth2?'))) {
        this.emailTransport.auth.type = 'OAuth2'
        do {
          this.emailTransport.auth.clientId = await prompt(
            chalk.green('Enter email OAuth2 clientId: ')
          )
          answer = await confirm(
            chalk.green(`Is this correct: "${this.emailTransport.auth.clientId}"`)
          )
          if (!answer) console.log("No problem, let's try again")
        } while (!answer)
        do {
          this.emailTransport.auth.clientSecret = await prompt(
            chalk.green('Enter email OAuth2 clientSecret: ')
          )
          answer = await confirm(
            chalk.green(`Is this correct: "${this.emailTransport.auth.clientSecret}"`)
          )
          if (!answer) console.log("No problem, let's try again")
        } while (!answer)
        do {
          this.emailTransport.auth.refreshToken = await prompt(
            chalk.green('Enter email OAuth2 refreshToken: ')
          )
          answer = await confirm(
            chalk.green(`Is this correct: "${this.emailTransport.auth.refreshToken}"`)
          )
          if (!answer) console.log("No problem, let's try again")
        } while (!answer)
      }
      const transporter = nodemailer.createTransport(this.emailTransport)

      spinner.setSpinnerTitle(chalk.yellow('Validating email settings... %s'))
      spinner.start()
      await sleep(1000)
      const emailStatus = await transporter.verify()

      readline.cursorTo(process.stdout, 0)
      console.log(chalk.green('Validating email settings... (done)'))
      spinner.stop()
      if (emailStatus === true) {
        console.log(chalk.green('Email settings validated and operational.'))
      } else {
        console.log(chalk.red(emailStatus))
        return
      }
    } else {
      this.emailService = 'None'
    }
    for await (const strategy of this.strategies) {
      strategy.value = await confirm(chalk.green(`Allow strategy: "${strategy.name}"?`))
    }
    answer = await confirm(chalk.green(`Change session TTL? (${prettyPrintMS(this.sessionTTL)})`))
    if (answer) {
      do {
        this.sessionTTL = parseInt(await prompt(chalk.green('Enter session TTL(in ms): ')))
        if (isNaN(this.sessionTTL)) this.sessionTTL = defaultSessionTTL
        answer = await confirm(chalk.green(`Is this correct: "${prettyPrintMS(this.sessionTTL)}"`))
        if (!answer) console.log("No problem, let's try again.")
      } while (!answer)
    }
    answer = await confirm(
      chalk.green(`Change two-factor TTL? (${prettyPrintMS(this.twoFactorTTL)})`)
    )
    if (answer) {
      do {
        this.twoFactorTTL = parseInt(await prompt(chalk.green('Enter two-factor TTL(in ms): ')))
        if (isNaN(this.twoFactorTTL)) this.twoFactorTTL = defaultTwoFactorTTL
        answer = await confirm(
          chalk.green(`Is this correct: "${prettyPrintMS(this.twoFactorTTL)}"`)
        )
        if (!answer) console.log("No problem, let's try again.")
      } while (!answer)
    }
  }

  getStrategyString(index) {
    return this.strategies[index].value
      ? chalk.green(`${this.strategies[index].name} ✔`)
      : chalk.red(`${this.strategies[index].name} ✘`)
  }

  getSessionTTL() {
    return chalk.green(`Session TTL: ${prettyPrintMS(this.sessionTTL)}`)
  }

  getTwoFactorTTL() {
    return chalk.green(`Two Factor TTL: ${prettyPrintMS(this.twoFactorTTL)}`)
  }

  getServerEmail() {
    if (this.emailService === 'None') {
      return chalk.green('Server Email: ') + chalk.red('Disabled')
    }
    return chalk.green('Server Email: ') + chalk.white(this.emailTransport.auth.user)
  }

  getVersionNumber() {
    if (!fs.existsSync('./package.json')) {
      console.log('Broker not installed properly, "package.json" missing.')
      process.exit(1)
    }
    const { version } = JSON.parse(fs.readFileSync('./package.json'))
    return chalk.green(`Broker v${version}`)
  }

  async getAddress() {
    this.externalIP = await fetch('http://checkip.amazonaws.com/').then((res) => res.text())
    this.externalIP = this.externalIP.replace(/\r?\n|\r/g, '')
    const users = await Promise.all(
      ((await this.db.getItem('users')) || []).map((u) => this.db.getItem(`user:${u.name}`))
    )

    users.forEach((user) => {
      if (user) {
        sendMail({
          broker: this,
          to: user.email,
          title: 'Server Restart',
          body: 'Credential Broker service restarted with ip: ' + this.externalIP
        })
      }
    })
    return chalk.green('External Address: ') + chalk.white(`${this.externalIP}:${this.port}`)
  }
}
