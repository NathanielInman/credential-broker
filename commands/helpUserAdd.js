import chalk from 'chalk'

export function helpUserAdd() {
  console.log(chalk.green('Command: ') + chalk.white('user add $username'))
  console.log(
    chalk.green('Brief: ') + chalk.white('Create a new user if the operator has edit access')
  )
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      "  This command allows creation of a new user account on the broker service if the operator has sufficient access to edit users. If the user already exists it asks if the operator wants to overwrite the existing user. If the user doesn't exist it asks for the users public PGP key. It then asks if the user should have access to view or edit other users. It then asks if the user should have access to viewing all scope names. It then asks if user should have access to create scopes. It then provides a list of scopes that currently exist allowing the operator to provide access to any scopes the user should have access to. For each scope the operator grants the user access, it asks whether that access should be a view-only or edit access."
    )
  )
}
