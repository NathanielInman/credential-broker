const chalk = require('chalk');

function getVersionNumber(){
  return chalk.green('Broker v1.0.0');
} //end getVersionNumber()

function getLoggedInRemoteIP(){
  return chalk.green('Remote Server IP: ')+chalk.white('Not set');
} //end getLoggedInRemoteIP()

function getLoggedInUsername(){
  return chalk.green('Current User: ')+chalk.white('Not logged in');
} //end getLoggedInUsername()

function getLoggedInEmail(){
  return chalk.green('Current Email: ')+chalk.white('Not logged in');
} //end getLoggedInEmail()

function getLastAuthenticationDate(){
  return chalk.green('Last Authentication: ')+chalk.white('Never');
} //end getLastAuthenticationDate()

function getLastAction(){
  return chalk.green('Last Action: ')+chalk.white('None');
} //end getLastAction()

function getLastScope(){
  return chalk.green('Last Scope: ')+chalk.white('None');
} //end getLastScope()

module.exports = {
  help(){
    console.log(chalk.blue( '         ./*.           '));
    console.log(chalk.blue( '     /@@@@@@@@@@.       '));
    console.log(chalk.blue( '   ,@@@@,    /@@@@      '));
    console.log(chalk.blue( '  .@@@.        *@@@     '));
    console.log(chalk.blue( '  @@@,          &@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@, ')+getVersionNumber());
    console.log(chalk.blue( '@@@@@@@@@@%%@@@@@@@@@@, ')+getLoggedInRemoteIP());
    console.log(chalk.blue( '@@@@@@@@#    ,@@@@@@@@, ')+getLoggedInUsername());
    console.log(chalk.blue( '@@@@@@@@&    #@@@@@@@@, ')+getLoggedInEmail());
    console.log(chalk.blue( '@@@@@@@@@.   @@@@@@@@@, ')+getLastAuthenticationDate());
    console.log(chalk.blue( '@@@@@@@@@@#(@@@@@@@@@@, ')+getLastAction());
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@  ')+getLastScope());
    console.log('');
    console.log(chalk.green('Commands: (')+chalk.white('help $command')+chalk.green(' for more information)'));
    console.log(chalk.white('  start                      ')+chalk.green('start broker server'));
    console.log(chalk.white('  init                       ')+chalk.green('initialize client and log into user'));
    console.log(chalk.white('  user add $username         ')+chalk.green('create a new user'));
    console.log(chalk.white('  user del $username         ')+chalk.green('delete an existing user'));
    console.log(chalk.white('  user mod $username         ')+chalk.green('modify an existing user'));
    console.log(chalk.white('  user get                   ')+chalk.green('get a list of all users'));
    console.log(chalk.white('  user get $username         ')+chalk.green('get information on existing user'));
    console.log(chalk.white('  scope add $scopename       ')+chalk.green('create a new scope'));
    console.log(chalk.white('  scope del $scopename       ')+chalk.green('delete an existing scope'));
    console.log(chalk.white('  scope mod $scopename       ')+chalk.green('modify an existing scope'));
    console.log(chalk.white('  scope get                  ')+chalk.green('get a list of all scope names'));
    console.log(chalk.white('  get                        ')+chalk.green('get a list of all scope names'));
    console.log(chalk.white('  add $scopename $secretname ')+chalk.green('create a new secret on existing scope'));
    console.log(chalk.white('  del $scopename $secretname ')+chalk.green('delete an existing secret on existing scope'));
    console.log(chalk.white('  mod $scopename $secretname ')+chalk.green('modify an existing secret on existing scope'));
    console.log(chalk.white('  get $scopename             ')+chalk.green('get all secrets from an existing scope'));
  }
};
