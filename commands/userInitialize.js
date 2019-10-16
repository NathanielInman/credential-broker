const fs = require('fs');
const openpgp = require('openpgp');
const crypto = require('crypto');
const fetch = require('node-fetch');
const {password,prompt,confirm} = require('../libraries/prompt.js');
const {spinner} = require('../libraries/spinner.js');
const {sleep} = require('../libraries/sleep.js');
const readline = require('readline');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {authSecure,authSecureEncrypt,authSecureDecrypt} = require('../libraries/authSecure.js');

module.exports = {
  async userInitialize(){
    let user;

    if(fs.existsSync('./user.json')){
      user = new User(JSON.parse(fs.readFileSync('./user.json')));
      if(await confirm(chalk.green('User (')+chalk.magenta(user.name)+chalk.green(') exists already, remove?'))){
        fs.unlinkSync('./user.json');
      }else{
        console.log(chalk.red('Canceling initialization'));
        process.exit(0);
      } //end if
    }else{
      user = new User();
    } //end if
    let answer;

    do{
      user.remoteIP = await prompt(chalk.green('Broker service url: '));
      if(!user.remoteIP.match(/^(http|https):\/\//g)){
        console.log(chalk.green('Missing http(s) in url, let\'s try again.'));
      }else{
        answer = await confirm(chalk.green(`Is this correct: "${user.remoteIP}"?`));
        if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
      } //end if
    }while(!answer)
    do{
      user.name = await prompt(chalk.green('Please enter name: '));
      answer = await confirm(chalk.green(`Is this correct: "${user.name}"?`));
      if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
    }while(!answer)
    do{
      user.email = await prompt(chalk.green('Please enter email: '));
      answer = await confirm(chalk.green(`Is this correct: "${user.email}"?`));
      if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
    }while(!answer)
    answer = await confirm(chalk.green('Establish a password? (Will be asked on every action)'));
    let passphrase,passphraseConfirm;

    if(answer){
      user.usePassword = true;

      do{
        passphrase = await password(chalk.green('Please enter password: '));
        passphraseConfirm = await password(chalk.green('Please confirm password: '));
        answer = passphrase===passphraseConfirm;
        if(!answer){
          console.log(chalk.red('Passwords didn\'t match, let\'s try again.'));
        } //end if
      }while(!answer)
    }else{
      user.usePassword = false;
      passphrase=user.email;
    } //end if
    spinner.setSpinnerTitle(chalk.yellow('Generating keys... %s'));
    spinner.start();
    await sleep(1000);
    const key = await openpgp.generateKey({
      userIds: `${user.name} <${user.email}>`,
      passphrase
    });
    readline.cursorTo(process.stdout, 0);
    console.log(chalk.green('Generating keys... (done)'));
    spinner.stop();
    spinner.setSpinnerTitle(chalk.yellow('Writing public key... %s'));
    spinner.start();
    await sleep(1000);
    fs.writeFileSync('./id_rsa.pub',key.publicKeyArmored);
    await sleep(1000);
    readline.cursorTo(process.stdout, 0);
    console.log(chalk.green('Writing public key... (done)'));
    spinner.stop();
    spinner.setSpinnerTitle(chalk.yellow('Writing private key... %s'));
    spinner.start();
    await sleep(1000);
    fs.writeFileSync('./id_rsa',key.privateKeyArmored);
    await sleep(1000);
    readline.cursorTo(process.stdout, 0);
    console.log(chalk.green('Writing private key... (done)'));
    spinner.stop();
    spinner.setSpinnerTitle(chalk.yellow('Writing revocation certificate... %s'));
    spinner.start();
    await sleep(1000);
    fs.writeFileSync('./id_rsa.revocationCertificate',key.revocationCertificate);
    await sleep(1000);
    spinner.stop();
    readline.cursorTo(process.stdout, 0);
    console.log(chalk.green('Writing revocation certificate... (done)'));
    user.lastAction = 'initialization';
    user.permissions.viewUsers = await confirm(chalk.green('Request view all users permission? '));
    user.permissions.editUsers = await confirm(chalk.green('Request edit all users permission? '));
    user.permissions.viewScopeNames = await confirm(chalk.green('Request view all scopes permission? '));
    user.permissions.createScopes = await confirm(chalk.green('Request create scopes permission? '));
    do{
      answer = await prompt(chalk.green('Enter requested scopes to access separated by commas: '));
      const scopeNames = answer.split(',').map(n=> n.trim()).filter(o=>o.length);

      for(let i=0;i<scopeNames.length;i++){
        answer = await confirm(chalk.green(`Request full access (Y) or view access to scope "${scopeNames[i]}"(n)?`));
        user.permissions.scopes.push({name: scopeNames[i],value: answer?'edit':'view'})
      } //end for
      if(!scopeNames.length){
        answer = await confirm(chalk.green('Proceed without requesting scope access?'));
      }else{
        answer = await confirm(chalk.green(`Is this correct: "${
          user.permissions.scopes.map(s=> `(${s.name}): ${s.value}`).join()
        }"?`));
      } //end if
      if(!answer){
        user.permissions.scopes.length = 0;
        console.log(chalk.green('No problem, let\'s try again.'));
      } //end if
    }while(!answer)

    // we initialize a secure transmission path by using diffie-hellman
    // to create an asymetric shared key that's used to encrypt traffic.
    if(!user.secret) user.secret = await authSecure(user);

    try{
      spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
      spinner.start();
      await fetch(`${user.remoteIP}/userInitialize`,{
        method: 'POST',
        body: JSON.stringify({
          ...user,
          key: fs.readFileSync('./id_rsa.pub').toString()
        }),
        headers: {
          'Content-Type': 'application/json',
          key: crypto.createHash('md5').update(user.name).digest('hex'),
          name: authSecureEncrypt(user.secret,user.name),
          email: authSecureEncrypt(user.secret,user.email)
        }
      })
        .then(res=> res.text())
        .then(res=> JSON.parse(authSecureDecrypt(user.secret,res)))
        .then(res=>{
          spinner.stop();
          readline.cursorTo(process.stdout, 0);
          console.log(chalk.green('Synchronizing with server... (done)'));
          if(res.success){
            fs.writeFileSync('./user.json',JSON.stringify(user));
            console.log(chalk.green('Initialization success!'));
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
