const chalk = require('chalk');
const {User} = require('../models/User.js');
const {request} = require('../libraries/request.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async userAdd(name){
    const newUser = new User({name});

    let answer;

    do{
      answer = await confirm(chalk.green(`Is this correct, create user: "${newUser.name}"?`));
      if(!answer){
        console.log(chalk.green('No problem, let\'s try again.'));
        newUser.name = await prompt(chalk.green('Please enter name: '));
      } //end if
    }while(!answer)
    do{
      newUser.email = await prompt(chalk.green('Please enter email: '));
      answer = await confirm(chalk.green(`Is this correct: "${newUser.email}"?`));
      if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
    }while(!answer)
    newUser.publicKey = await prompt(chalk.green('Please enter public key: '));
    newUser.permissions.viewUsers = await confirm(chalk.green('Allow view all users permission? '));
    newUser.permissions.editUsers = await confirm(chalk.green('Allow edit all users permission? '));
    newUser.permissions.viewScopeNames = await confirm(chalk.green('Allow view all scopes permission? '));
    newUser.permissions.createScopes = await confirm(chalk.green('Allow create scopes permission? '));
    do{
      answer = await prompt(chalk.green('Enter scopes to access separated by commas: '));
      const scopeNames = answer.split(',').map(n=> n.trim());

      for(let i=0;i<scopeNames.length;i++){
        answer = await confirm(chalk.green(`Allow full access (Y) or view access to scope "${scopeNames[i]}"(n)?`));
        newUser.permissions.scopes.push({name: scopeNames[i],value: answer?'edit':'view'});
      } //end for
      answer = await confirm(chalk.green(`Is this correct: "${
        newUser.permissions.scopes.map(s=> `(${s.name}): ${s.value}`).join()
      }"?`));
      if(!answer){
        newUser.permissions.scopes.length = 0;
        console.log(chalk.green('No problem, let\'s try again.'));
      } //end if
    }while(!answer)
    try{
      const res = await request('userAdd',newUser);

      if(res.success){
        console.log(chalk.green(`User "${newUser.name}" added successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
