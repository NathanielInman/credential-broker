const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async scopeModify(name){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/scopeGet`,{
              method: 'POST',
              body: JSON.stringify({name}),
              headers: {
                'Content-Type': 'application/json',
                key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
                name: user.name,
                email: user.email
              }
            })
            .then(res=> res.json());

      if(data.error) return console.log(chalk.red(res.error));
      const updatedScope = data.success;

      let answer,bool;

      do{
        answer = updatedScope.name;
        bool = await confirm(chalk.green(`Keep name "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Please enter name: '));
          bool = await confirm(chalk.green(`Is this correct: "${answer}"?`));
        } //end if
      }while(!bool)
      updatedScope.name = answer;
      if(updatedScope.publicKey){
        bool = await confirm(chalk.green(`Continue allowing scope to be a user?`));
        if(!bool){
          updatedScope.publicKey = '';
        }else if(await confirm(chalk.green('Change existing public key?'))){
          updatedScope.publicKey = prompt(chalk.green('Please enter public key: '));
        } //end if
      }else if(!(await confirm(chalk.green(`Continue preventing scope from being a user?`)))){
        updatedScope.publicKey = prompt(chalk.green('Please enter public key: '));
      } //end if
      await fetch(`${user.remoteIP}/scopeModify`,{
        method: 'POST',
        body: JSON.stringify({
          target: name,
          scopeName: updatedScope.scopeName,
          scopePublicKey: updatedScope.scopePublicKey
        }),
        headers: {
          'Content-Type': 'application/json',
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(res.success){
            console.log(chalk.green(`Scope "${scopeName}" modified successfully!`));
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
