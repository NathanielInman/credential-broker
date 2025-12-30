import chalk from 'chalk'

export function helpScopeAdd() {
  console.log(chalk.green('Command: ') + chalk.white('scope add $scopename'))
  console.log(
    chalk.green('Brief: ') + chalk.white('Create a scope if user has scope create access')
  )
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      '  An operator may create a scope if they have scope create access. The command starts by validating that the user indeed has access to create the scope first, and then asks if the scope itself can be a user. If the scope can be a user it then asks for the public PGP key so that the application can access its own scope by using the broker service.'
    )
  )
}
