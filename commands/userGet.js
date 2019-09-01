const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');

module.exports = {
  async userGet(username){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/userGet`,{
        method: 'POST',
        body: JSON.stringify({
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
          username
        }),
        headers: {
          'Content-Type': 'application/json',
          username: user.username,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(username===user.username){
            console.log(chalk.magenta('[CLIENT]'));
            console.log(chalk.cyan('remoteIP: ')+chalk.green(user.remoteIP));
            console.log(chalk.cyan('username: ')+chalk.green(user.username));
            console.log(chalk.cyan('email: ')+chalk.green(user.email));
            console.log(chalk.cyan('pgpPrivateKeyLocation: ')+chalk.green(user.pgpPrivateKeyLocation));
            console.log(chalk.cyan('pgpPublicKeyLocation: ')+chalk.green(user.pgpPublicKeyLocation));
            console.log(chalk.cyan('lastAuthentication: ')+chalk.green(user.lastAuthentication));
            console.log(chalk.cyan('lastAction: ')+chalk.green(user.lastAction));
            console.log(chalk.cyan('lastScope: ')+chalk.green(user.lastScope));
            console.log(chalk.cyan('permissions:'));
            console.log(chalk.cyan('  viewUsers: ')+chalk.green(user.permissions.viewUsers));
            console.log(chalk.cyan('  editUsers: ')+chalk.green(user.permissions.editUsers));
            console.log(chalk.cyan('  viewScopenames: ')+chalk.green(user.permissions.viewScopeNames));
            console.log(chalk.cyan('  createScopes: ')+chalk.green(user.permissions.createScopes));
            console.log(chalk.cyan('scopes:'));
            user.permissions.scopes.forEach(scope=>{
              console.log(chalk.cyan(`  ${scope.name}: `)+chalk.green(scope.value));
            });
          } //end if
          if(res.success){
            console.log(chalk.magenta('[SERVER]'));
            console.log(chalk.cyan('date: ')+chalk.green(res.success.date));
            console.log(chalk.cyan('username: ')+chalk.green(res.success.username));
            console.log(chalk.cyan('addedBy: ')+chalk.green(res.success.addedBy));
            console.log(chalk.cyan('addedByIP: ')+chalk.green(res.success.addedByIP));
            console.log(chalk.cyan('publicKey: ')+chalk.green(res.success.key));
            console.log(chalk.cyan('permissions:'));
            console.log(chalk.cyan('  viewUsers: ')+chalk.green(res.success.permissions.viewUsers));
            console.log(chalk.cyan('  editUsers: ')+chalk.green(res.success.permissions.editUsers));
            console.log(chalk.cyan('  viewScopeNames: ')+chalk.green(res.success.permissions.viewScopeNames));
            console.log(chalk.cyan('  createScopes: ')+chalk.green(res.success.permissions.createScopes));
            console.log(chalk.cyan('scopes:'));
            res.success.permissions.scopes.forEach(scope=>{
              console.log(chalk.cyan(`  ${scope.name}: `)+chalk.green(scope.value));
            });
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
