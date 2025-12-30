import chalk from 'chalk'
import { request } from '../libraries/request.js'

export async function scopeGetAll() {
  try {
    const { user, ...res } = await request('scopeGetAll')

    if (res.success && res.success.length) {
      console.log(chalk.green(res.success))
    } else if (res.success) {
      console.log(chalk.cyan("No scopes exist or you don't have access to view them."))
    } else {
      console.log(chalk.red(res.error))
    } // end if
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
