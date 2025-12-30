import chalk from 'chalk'

export function helpModify() {
  console.log(chalk.green('Command: ') + chalk.white('mod $scopename $secretname'))
  console.log(chalk.green('Brief: ') + chalk.white('Modify a secret on a scope'))
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      "  An operator may change a secret on a specified scope if the operator has edit access to the scope, the scope exists and the secret exists. It firsts asks that the operator intends to change the specified secret, and then asks for the value while hiding the input. It immediately encrypts the value and transmits it to the broker service. It's recommended to setup the broker service with TLS."
    )
  )
}
