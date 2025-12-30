import chalk from 'chalk'
import { prompt, confirm } from '../libraries/prompt.js'
import { request } from '../libraries/request.js'

export async function scopeAdd(name) {
  let scopeName = name
  let scopePublicKey = ''
  let bool

  do {
    bool = await confirm(chalk.green(`Is this correct, add scope: "${scopeName}"?`))
    if (!bool) {
      console.log(chalk.green("No problem, let's try again."))
      scopeName = await prompt(chalk.green('Enter new scope name: '))
    } // end if
  } while (!bool)
  do {
    bool = await confirm(chalk.green(`Will scope "${scopeName}" also be a user?`))
    if (bool) {
      scopePublicKey = await prompt(chalk.green('Enter public PGP key for scope: '))
    } // end if
    bool = true
  } while (!bool)
  try {
    const { user, ...res } = await request('scopeAdd', { scopeName, scopePublicKey })

    if (res.success) {
      console.log(chalk.green(`Scope "${scopeName}" added successfully!`))
    } else {
      console.log(chalk.red(res.error))
    } // end if
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
