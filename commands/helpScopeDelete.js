const chalk = require('chalk');

module.exports = {
  helpScopeDelete(){
    console.log(chalk.green('Command: ')+chalk.white('scope del $scopename'));
    console.log(chalk.green('Brief: ')+chalk.white('Delete an existing scope if the user has scope edit access'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  An operator may delete an existing scope if they have edit access to that scope.'));
  }
}
