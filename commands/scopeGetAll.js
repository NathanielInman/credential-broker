const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const readline = require('readline');
const crypto = require('crypto');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');
const {decrypt} = require('../libraries/decrypt.js');
const {spinner} = require('../libraries/spinner.js');
const {authSecureEncrypt,authSecureDecrypt} = require('../libraries/authSecure.js');

module.exports = {
  async scopeGetAll(){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
      spinner.start();
      await fetch(`${user.remoteIP}/scopeGetAll`,{
        method: 'POST',
        body: '',
        headers: {
          'Content-Type': 'text/plain',
          key: crypto.createHash('md5').update(user.name).digest('hex'),
          name: authSecureEncrypt(user.secret,user.name),
          email: authSecureEncrypt(user.secret,user.email)
        }
      })
        .then(res=> res.text())
        .then(res=>{
          console.log('decrypting',res);
          return JSON.parse(authSecureDecrypt(user.secret,res));
        })
        .then(async res=>{
          console.log('decrypted',res);

          spinner.stop();
          readline.cursorTo(process.stdout, 0);
          console.log(chalk.green('Synchronizing with server... (done)'));
          if(res.success&&res.success.length){
            console.log(chalk.green(await decrypt(user,res.success)));
          }else if(res.success){
            console.log(chalk.cyan('No scopes exist or you don\'t have access to view them.'));
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
