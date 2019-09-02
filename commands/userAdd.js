const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async userAdd(name){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json'))),
          newUser = JSON.parse(JSON.stringify(user));

    newUser.name = name;
    newUser.permissions.scopes.length=0;
    let answer;

    do{
      answer = await confirm(`Is this correct, create user: "${newUser.name}"?`);
      if(!answer){
        console.log('No problem, let\'s try again.');
        newUser.name = await prompt('Please enter name: ');
      } //end if
    }while(!answer)
    do{
      newUser.email = await prompt('Please enter email: ');
      answer = await confirm(`Is this correct: "${newUser.email}"?`);
      if(!answer) console.log('No problem, let\'s try again.');
    }while(!answer)
    newUser.publicKey = await prompt('Please enter public key: ');
    newUser.permissions.viewUsers = await confirm('Allow view all users permission? ');
    newUser.permissions.editUsers = await confirm('Allow edit all users permission? ');
    newUser.permissions.viewScopeNames = await confirm('Allow view all scopes permission? ');
    newUser.permissions.createScopes = await confirm('Allow create scopes permission? ');
    do{
      answer = await prompt('Enter scopes to access separated by commas: ');
      const scopeNames = answer.split(',').map(n=> n.trim());

      for(let i=0;i<scopeNames.length;i++){
        answer = await confirm(`Allow full access (Y) or view access to scope "${scopeNames[i]}"(n)?`);
        newUser.permissions.scopes.push({name: scopeNames[i],value: answer?'edit':'view'});
      } //end for
      answer = await confirm(`Is this correct: "${
        newUser.permissions.scopes.map(s=> `(${s.name}): ${s.value}`).join()
      }"?`);
      if(!answer){
        newUser.permissions.scopes.length = 0;
        console.log('No problem, let\'s try again.');
      } //end if
    }while(!answer)
    try{
      await fetch(`${user.remoteIP}/userAdd`,{
        method: 'POST',
        body: JSON.stringify({
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
          ...newUser
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
