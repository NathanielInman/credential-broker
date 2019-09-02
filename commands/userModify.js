const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async userModify(name){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/userGet`,{
              method: 'POST',
              body: JSON.stringify({
                key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
                name
              }),
              headers: {
                'Content-Type': 'application/json',
                name: user.name,
                email: user.email
              }
            })
            .then(res=> res.json());

      if(data.error) return console.log(chalk.red(res.error));
      const updatedUser = data.success;

      let answer,bool;

      do{
        answer = updatedUser.name;
        bool = await confirm(`Keep name "${answer}"?`);
        if(!bool){
          answer = await prompt('Please enter name: ');
          bool = await confirm(`Is this correct: "${answer}"?`);
        } //end if
      }while(!bool)
      updatedUser.name = answer;
      do{
        answer = updatedUser.email;
        bool = await confirm(`Keep email "${answer}"?`);
        if(!bool){
          answer = await prompt('Please enter email: ');
          bool = await confirm(`Is this correct: "${answer}"?`);
        } //end if
      }while(!bool)
      updatedUser.email = answer;
      do{
        answer = updatedUser.publicKey;
        bool = await confirm(`Keep existing public key?`);
        if(!bool){
          answer = await prompt('Please enter new public key: ');
          bool = await confirm(`Is this correct: "${answer}"?`);
        } //end if
      }while(!bool)
      updatedUser.publicKey = answer;
      do{
        answer = updatedUser.permissions.viewUsers;
        bool = await confirm(`Keep view all users permission "${answer}"?`);
        if(!bool){
          answer = await prompt('Allow view all users permission? ');
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.viewUsers = answer;
      do{
        answer = updatedUser.permissions.editUsers;
        bool = await confirm(`Keep edit all users permission "${answer}"?`);
        if(!bool){
          answer = await prompt('Allow edit all users permission? ');
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.editUsers = answer;
      do{
        answer = updatedUser.permissions.viewScopeNames;
        bool = await confirm(`Keep view all scopes permission "${answer}"?`);
        if(!bool){
          answer = await prompt('Allow view all scopes permission? ');
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.viewScopeNames = answer;
      do{
        answer = updatedUser.permissions.createScopes;
        bool = await confirm(`Keep create scopes permission "${answer}"?`);
        if(!bool){
          answer = await prompt('Allow create scopes permission? ');
          bool = true;
        } //end if
      }while(!bool)
      updatedUser.permissions.createScopes = answer;
      for(let i=0;i<updatedUser.permissions.scopes.length;i++){
        let scopeName = updatedUser.permissions.scopes[i].name;

        do{
          answer = updatedUser.permissions.scopes[i].value;
          bool = await confirm(`Keep scope "${scopeName}" access "${answer}"?`);
          if(!bool){
            answer = await prompt(`Allow full access (Y) or view access to scope "${scopeName}"?`);
            bool = true;
          } //end if
        }while(!bool)
        updatedUser.permissions.scopes[i].value = answer?'edit':'view';
      } //end for
      await fetch(`${user.remoteIP}/userModify`,{
        method: 'POST',
        body: JSON.stringify({
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
          ...updatedUser
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
            console.log('User modification success!');
          }else{
            console.log(res.error);
          } //end if
        });

    }catch(err){
      console.log('Problem connecting to server.');
      console.log(err);
    }
  }
};
