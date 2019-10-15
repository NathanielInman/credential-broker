const chalk = require('chalk');
const {request} = require('../libraries/request.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async userModify(name){
    try{
      const data = await request('userGet',{name});

      if(data.error) return console.log(chalk.red(res.error));
      const updatedUser = data.success;

      let answer,bool;

      do{
        answer = updatedUser.name;
        bool = await confirm(chalk.green(`Keep name "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Please enter name: '));
          bool = await confirm(chalk.green(`Is this correct: "${answer}"?`));
        } //end if
      }while(!bool)
      updatedUser.name = answer;
      do{
        answer = updatedUser.email;
        bool = await confirm(chalk.green(`Keep email "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Please enter email: '));
          bool = await confirm(chalk.green(`Is this correct: "${answer}"?`));
        } //end if
      }while(!bool)
      updatedUser.email = answer;
      do{
        answer = updatedUser.publicKey;
        bool = await confirm(chalk.green(`Keep existing public key?`));
        if(!bool){
          answer = await prompt(chalk.green('Please enter new public key: '));
          bool = await confirm(chalk.green(`Is this correct: "${answer}"?`));
        } //end if
      }while(!bool)
      updatedUser.publicKey = answer;
      do{
        answer = updatedUser.permissions.viewUsers;
        bool = await confirm(chalk.green(`Keep view all users permission "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Allow view all users permission? '));
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.viewUsers = answer;
      do{
        answer = updatedUser.permissions.editUsers;
        bool = await confirm(chalk.green(`Keep edit all users permission "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Allow edit all users permission? '));
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.editUsers = answer;
      do{
        answer = updatedUser.permissions.viewScopeNames;
        bool = await confirm(chalk.green(`Keep view all scopes permission "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Allow view all scopes permission? '));
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.viewScopeNames = answer;
      do{
        answer = updatedUser.permissions.createScopes;
        bool = await confirm(chalk.green(`Keep create scopes permission "${answer}"?`));
        if(!bool){
          answer = await prompt(chalk.green('Allow create scopes permission? '));
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.createScopes = answer;
      for(let i=0;i<updatedUser.permissions.scopes.length;i++){
        let scopeName = updatedUser.permissions.scopes[i].name;

        do{
          answer = updatedUser.permissions.scopes[i].value;
          bool = await confirm(chalk.green(`Keep scope "${scopeName}" access "${answer}"?`));
          if(!bool){
            answer = await prompt(chalk.green(`Allow full access (Y) or view access to scope "${scopeName}"?`));
            bool = true;
          } //end if
        }while(!bool)
        updatedUser.permissions.scopes[i].value = answer?'edit':'view';
      } //end for
      const res = await request('userModify',updatedUser);

      if(res.success){
        console.log(chalk.green(`User "${updatedUser.name}" modified successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
