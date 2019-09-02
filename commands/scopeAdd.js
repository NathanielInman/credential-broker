const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async scopeAdd(name){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    let scopeName=name,scopePublicKey='',bool;

    do{
      bool = await confirm(`Is this correct: "${answer}"?`);
      if(!bool){
        console.log('No problem, let\'s try again.');
        scopeName = await prompt('Enter new scope name: ');
      } //end if
    }while(!bool)
    do{
      bool = await confirm(`Will scope "${answer}" also be a user?`);
      if(bool){
        scopePublicKey = await prompt('Enter public PGP key for scope: ');
      } //end if
      bool = true;
    }while(!bool)
    try{
      await fetch(`${user.remoteIP}/scopeAdd`,{
        method: 'POST',
        body: JSON.stringify({
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
          scopeName,
          scopePublicKey
        }),
        headers: {
          'Content-Type': 'application/json',
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(res.success){
            console.log(chalk.green(res.success));
          }else{
            console.log(chalk.red(res.error));
          } //end if
        });
    }catch(err){
      console.log('Problem connecting to server.');
      console.log(err);
    }
  }
};
