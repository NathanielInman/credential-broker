import chalk from 'chalk'
import { request } from '../libraries/request.js'
import { confirm } from '../libraries/prompt.js'

export async function secretDelete(scopeName, secretName) {
  const test = await confirm(chalk.green(`Delete "${secretName}" from scope "${scopeName}"?`))

  if (!test) return
  try {
    const res = await request('secretDelete', { scopeName, secretName })

    if (res.success) {
      console.log(chalk.green(`Secret "${secretName}" from "${scopeName}" deleted successfully!`))
    } else {
      console.log(chalk.red(res.error))
    } // end if
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
