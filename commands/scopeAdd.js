const chalk = require('chalk');
const {prompt,confirm} = require('../libraries/prompt.js');
const {request} = require('../libraries/request.js');

module.exports = {
  async scopeAdd(name){
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
      const {user,...res} = request('scopeAdd',{scopeName,scopePublicKey});

      if(res.success){
        console.log(chalk.green(`Scope "${scopeName}" added successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
