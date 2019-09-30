const fs = require('fs');
const {confirm} = require('../libraries/prompt.js');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {mail} = require('../libraries/mail.js');

    mail({
      from: 'nate@theoestudio.com', //req.broker.externalIP,
      to: 'nate@theoestudio.com',
      subject: 'Welcome to Credential Broker!',
      text: `Welcome to Crednetial Broker test!`
    });
module.exports = {
  async init(){
    if(fs.existsSync('./user.json')){
      const user = new User(JSON.parse(fs.readFileSync('./user.json')));

      if(await confirm(chalk.green('User (')+chalk.magenta(user.name)+chalk.green(') exists already, remove?'))){
        fs.unlinkSync('./user.json');
        await user.initialize();
      }else{
        console.log(chalk.red('Canceling initialization'));
        process.exit(0);
      } //end if
    }else{
      const user = new User();

      await user.initialize();
    } //end if
  }
};
