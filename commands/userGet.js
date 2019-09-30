const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const readline = require('readline');
const {User} = require('../models/User.js');
const {decrypt} = require('../libraries/decrypt.js');
const {sign} = require('../libraries/sign.js');
const {spinner} = require('../libraries/spinner.js');

module.exports = {
  async userGet(name){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
      spinner.start();
      await fetch(`${user.remoteIP}/userGet`,{
        method: 'POST',
        body: await sign(user,JSON.stringify({name})),
        headers: {
          'Content-Type': 'text/plain',
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(async res=>{
          spinner.stop();
          readline.cursorTo(process.stdout, 0);
          console.log(chalk.green('Synchronizing with server... (done)'));
          if(name===user.name){
            console.log(chalk.magenta('[CLIENT]'));
            console.log(chalk.cyan('remoteIP: ')+chalk.green(user.remoteIP));
            console.log(chalk.cyan('name: ')+chalk.green(user.name));
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
            const data = await decrypt(user,res.success);

            console.log(chalk.magenta('[SERVER]'));
            console.log(chalk.cyan('date: ')+chalk.green(data.date));
            console.log(chalk.cyan('name: ')+chalk.green(data.name));
            console.log(chalk.cyan('email: ')+chalk.green(data.email));
            console.log(chalk.cyan('addedBy: ')+chalk.green(data.addedBy));
            console.log(chalk.cyan('addedByIP: ')+chalk.green(data.addedByIP));
            console.log(chalk.cyan('publicKey: ')+chalk.green(data.key));
            console.log(chalk.cyan('permissions:'));
            console.log(chalk.cyan('  viewUsers: ')+chalk.green(data.permissions.viewUsers));
            console.log(chalk.cyan('  editUsers: ')+chalk.green(data.permissions.editUsers));
            console.log(chalk.cyan('  viewScopeNames: ')+chalk.green(data.permissions.viewScopeNames));
            console.log(chalk.cyan('  createScopes: ')+chalk.green(data.permissions.createScopes));
            console.log(chalk.cyan('scopes:'));
            data.permissions.scopes.forEach(scope=>{
              console.log(chalk.cyan(`  ${scope.name}: `)+chalk.green(scope.value));
            });
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
