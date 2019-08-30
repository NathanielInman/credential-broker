const chalk = require('chalk');

module.exports = {
  helpScopeModify(){
    console.log(chalk.green('Command: ')+chalk.white('scope mod $scopename'));
    console.log(chalk.green('Brief: ')+chalk.white('Modify a scope if the user has edit access to the scope'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  An operator may edit a scopes name or edit/add/remove a public PGP key on a scope if the scope exists and the operator has edit access to the scope.'));
  }
}
