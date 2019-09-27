const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');
const {sign} = require('../libraries/sign.js');
const {spinner} = require('../libraries/spinner.js');

module.exports = {
  async scopeAdd(name){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    let scopeName=name,scopePublicKey='',bool;

    do{
      bool = await confirm(chalk.green(`Is this correct, add scope: "${scopeName}"?`));
      if(!bool){
        console.log(chalk.green('No problem, let\'s try again.'));
        scopeName = await prompt(chalk.green('Enter new scope name: '));
      } //end if
    }while(!bool)
    do{
      bool = await confirm(chalk.green(`Will scope "${scopeName}" also be a user?`));
      if(bool){
        scopePublicKey = await prompt(chalk.green('Enter public PGP key for scope: '));
      } //end if
      bool = true;
    }while(!bool)
    try{
      spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
      spinner.start();
      await fetch(`${user.remoteIP}/scopeAdd`,{
        method: 'POST',
        body: await sign(user,JSON.stringify({scopeName,scopePublicKey})),
        headers: {
          'Content-Type': 'text/plain',
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          spinner.stop();
          readline.cursorTo(process.stdout, 0);
          console.log(chalk.green('Synchronizing with server... (done)'));
          if(res.success){
            console.log(chalk.green(`Scope "${scopeName}" added successfully!`));
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
