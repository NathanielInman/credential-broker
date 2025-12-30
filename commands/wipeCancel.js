import chalk from 'chalk'
import { request } from '../libraries/request.js'

export async function wipeCancel() {
  try {
    const res = await request('wipeCancel', {})

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
