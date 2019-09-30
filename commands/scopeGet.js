const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const readline = require('readline');
const {User} = require('../models/User.js');
const {decrypt} = require('../libraries/decrypt.js');
const {sign} = require('../libraries/sign.js');
const {spinner} = require('../libraries/spinner.js');

module.exports = {
  async scopeGet(scopeName){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
      spinner.start();
      await fetch(`${user.remoteIP}/scopeGet`,{
        method: 'POST',
        body: await sign(user,JSON.stringify({scopeName})),
        headers: {
          'Content-Type': 'text/plain',
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(async res=>{
          spinner.stop();
          readline.cursorTo(process.stdout, 0);
          console.log(chalk.green('Synchronizing with server... (done)'));
          if(res.success){
            console.log(chalk.green(await decrypt(user,res.success)));
          }else{
            console.log(chalk.red(res.error));
          } //end if
        });
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
