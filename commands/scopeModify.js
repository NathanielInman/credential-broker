const chalk = require('chalk');
const {request} = require('../libraries/request.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async scopeModify(name){
    try{
      const data = await request('scopeGet',{name});

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
      const result = await request('scopeModify',{
        target: name,
        scopeName: updatedScope.scopeName,
        scopePublicKey: updatedScope.scopePublicKey
      });

      if(res.success){
        console.log(chalk.green(`Scope "${scopeName}" modified successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
