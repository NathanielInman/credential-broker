import chalk from 'chalk'

export function helpDelete() {
  console.log(chalk.green('Command: ') + chalk.white('del $scopename $secretname'))
  console.log(chalk.green('Brief: ') + chalk.white('Delete a secret from a scope'))
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      '  An operator may delete a secret from a specified scope if the scope exists, the secret exists and the operator has edit access to the scope. Before deleting it validates with the operator that they indeed intend to delete the specified secret.'
    )
  )
}
