import chalk from 'chalk'

export function helpUserModify() {
  console.log(chalk.green('Command: ') + chalk.white('user mod $username'))
  console.log(
    chalk.green('Brief: ') +
      chalk.white('Modify an existing user if the operator has user edit access')
  )
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      '  An operator may modify any existing values on an existing user if they have user edit access. Upon using the command it asks if the user exists and validates that the operator indeed wants to modify the user first. It then asks if the operator wants to change the username and if so asks for an updated username. It then asks if the operator wants to change the public key of the user and if yes asks for an updated public PGP key. It then asks if the user should have access to view or edit other users. It then asks if the user should have access to viewing all scope names. It then asks if the user should have access to create scopes. It finally provides a list of all existing scopes, allowing the operator to select which scopes the user should be able to access and for each selected asks whether that access should be view-only or edit access.'
    )
  )
}
