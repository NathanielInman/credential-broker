import chalk from 'chalk'
import { request } from '../libraries/request.js'

export async function userGet(name) {
  try {
    const { user, ...res } = await request('userGet', { name })

    if (name === user.name) {
      console.log(chalk.magenta('[CLIENT]'))
      console.log(chalk.cyan('remoteIP: ') + chalk.green(user.remoteIP))
      console.log(chalk.cyan('name: ') + chalk.green(user.name))
      console.log(chalk.cyan('email: ') + chalk.green(user.email))
      console.log(chalk.cyan('pgpPrivateKeyLocation: ') + chalk.green(user.pgpPrivateKeyLocation))
      console.log(chalk.cyan('pgpPublicKeyLocation: ') + chalk.green(user.pgpPublicKeyLocation))
      console.log(chalk.cyan('lastAuthentication: ') + chalk.green(user.lastAuthentication))
      console.log(chalk.cyan('lastAction: ') + chalk.green(user.lastAction))
      console.log(chalk.cyan('lastScope: ') + chalk.green(user.lastScope))
      console.log(chalk.cyan('permissions:'))
      console.log(chalk.cyan('  viewUsers: ') + chalk.green(user.permissions.viewUsers))
      console.log(chalk.cyan('  editUsers: ') + chalk.green(user.permissions.editUsers))
      console.log(chalk.cyan('  viewScopenames: ') + chalk.green(user.permissions.viewScopeNames))
      console.log(chalk.cyan('  createScopes: ') + chalk.green(user.permissions.createScopes))
      console.log(chalk.cyan('scopes:'))
      user.permissions.scopes.forEach((scope) => {
        console.log(chalk.cyan(`  ${scope.name}: `) + chalk.green(scope.value))
      })
    } // end if
    if (res.success) {
      console.log(chalk.magenta('[SERVER]'))
      console.log(chalk.cyan('date: ') + chalk.green(res.success.date))
      console.log(chalk.cyan('name: ') + chalk.green(res.success.name))
      console.log(chalk.cyan('email: ') + chalk.green(res.success.email))
      console.log(chalk.cyan('addedBy: ') + chalk.green(res.success.addedBy))
      console.log(chalk.cyan('addedByIP: ') + chalk.green(res.success.addedByIP))
      console.log(chalk.cyan('publicKey: ') + chalk.green(res.success.key))
      console.log(chalk.cyan('permissions:'))
      console.log(chalk.cyan('  viewUsers: ') + chalk.green(res.success.permissions.viewUsers))
      console.log(chalk.cyan('  editUsers: ') + chalk.green(res.success.permissions.editUsers))
      console.log(
        chalk.cyan('  viewScopeNames: ') + chalk.green(res.success.permissions.viewScopeNames)
      )
      console.log(
        chalk.cyan('  createScopes: ') + chalk.green(res.success.permissions.createScopes)
      )
      console.log(chalk.cyan('scopes:'))
      res.success.permissions.scopes.forEach((scope) => {
        console.log(chalk.cyan(`  ${scope.name}: `) + chalk.green(scope.value))
      })
    } else {
      console.log(chalk.red(res.error))
    } // end if
  } catch (err) {
    console.log(chalk.red('Problem connecting to server.'))
    console.log(chalk.red(err))
  }
}
