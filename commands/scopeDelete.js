import chalk from 'chalk'
import { request } from '../libraries/request.js'

export async function scopeDelete(scopeName) {
  try {
    const res = await request('scopeDelete', { scopeName })

    if (res.success) {
      console.log(chalk.green(`Scope "${scopeName}" deleted successfully!`))
    } else {
      console.log(chalk.red(res.error))
    } // end if
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
