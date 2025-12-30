import chalk from 'chalk'
import { request } from '../libraries/request.js'
import { confirm } from '../libraries/prompt.js'

export async function wipe() {
  const test = await confirm(
    chalk.red('WARNING: This will remove ALL users from the broker. Continue?')
  )

  if (!test) return
  try {
    const res = await request('wipe', {})

    if (res.success) {
      console.log(chalk.green(res.success))
    } else {
      console.log(chalk.red(res.error))
    }
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
