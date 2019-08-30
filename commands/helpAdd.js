const chalk = require('chalk');

module.exports = {
  helpAdd(){
    console.log(chalk.green('Command: ')+chalk.white('add $scopename $secretname'));
    console.log(chalk.green('Brief: ')+chalk.white('Add a secret to a scope'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  An operator may add a secret to a specified scope if the scope exists and the user has scope edit access. The command will ask for the secret value on a separate line hiding input. The secret will be encrypted immediately before transmitting. It\'s recommended that the broker service is stood up on a server that talks over TLS.'));

  }
}
