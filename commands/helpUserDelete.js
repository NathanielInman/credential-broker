const chalk = require('chalk');

module.exports = {
  helpUserDelete(){
    console.log(chalk.green('Command: ')+chalk.white('user del $username'));
    console.log(chalk.green('Brief: ')+chalk.white('Delete an existing user if the operator has user edit access'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  An operator may delete a user from the remote broker service if that operator has user edit access.'));
  }
}
