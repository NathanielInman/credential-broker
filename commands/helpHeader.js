const chalk = require('chalk');
const fs = require('fs');
const {User} = require('../models/User.js');

let user;

if(!fs.existsSync('./user.json')){
  user = new User;
}else{
  user = new User(JSON.parse(fs.readFileSync('./user.json')));
} //end if

module.exports = {
  helpHeader(){
    console.log(chalk.blue( '         ./*.           '));
    console.log(chalk.blue( '     /@@@@@@@@@@.       '));
    console.log(chalk.blue( '   ,@@@@,    /@@@@      '));
    console.log(chalk.blue( '  .@@@.        *@@@     '));
    console.log(chalk.blue( '  @@@,          &@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@, ')+user.getVersionNumber());
    console.log(chalk.blue( '@@@@@@@@@@%%@@@@@@@@@@, ')+user.getLoggedInRemoteIP());
    console.log(chalk.blue( '@@@@@@@@#    ,@@@@@@@@, ')+user.getLoggedInUsername());
    console.log(chalk.blue( '@@@@@@@@&    #@@@@@@@@, ')+user.getLoggedInEmail());
    console.log(chalk.blue( '@@@@@@@@@.   @@@@@@@@@, ')+user.getLastAuthenticationDate());
    console.log(chalk.blue( '@@@@@@@@@@#(@@@@@@@@@@, ')+user.getLastAction());
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@  ')+user.getLastScope());
    console.log('');
  }
};
