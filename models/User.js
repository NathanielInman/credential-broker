const chalk = require('chalk');
const fs = require('fs');
const fetch = require('node-fetch');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  User: class User{
    constructor({
      remoteIP='http://localhost:4000',username='',email='',lastAuthentication=null,
      lastAction='',lastScope='',
      pgpPrivateKeyLocation='',pgpPublicKeyLocation=''
    }={}){
      this.remoteIP = remoteIP;
      this.username = username;
      this.email = email;
      this.pgpPrivateKeyLocation = pgpPrivateKeyLocation;
      this.pgpPublicKeyLocation = pgpPublicKeyLocation;
      this.lastAuthentication = lastAuthentication;
      this.lastAction = lastAction;
      this.lastScope = lastScope;
    }
    async initialize(){
      let answer;

      do{
        this.remoteIP = await prompt('Broker service ip: ');

        answer = await confirm(`Is this correct: "${this.remoteIP}"?`);
        if(!answer) console.log('No problem, let\'s try again.');
      }while(!answer)
      do{
        this.username = await prompt('Please enter username: ');

        answer = await confirm(`Is this correct: "${this.username}"?`);
        if(!answer) console.log('No problem, let\'s try again.');
      }while(!answer)
      do{
        this.email = await prompt('Please enter email: ');

        answer = await confirm(`Is this correct: "${this.email}"?`);
        if(!answer) console.log('No problem, let\'s try again.');
      }while(!answer)
      do{
        console.log('Note: This file will be sent to broker service to validate user identity.');
        this.pgpPublicKeyLocation = await prompt('Please enter file location including name of public PGP Key: ');
        answer = await confirm(`Is this correct: "${this.pgpPublicKeyLocation}"?`);
        if(!answer) console.log('No problem, let\'s try again.');
      }while(!answer)
      do{
        console.log('Note: this file will never be stored elsewhere, and is only used to validate identity.');
        this.pgpPrivateKeyLocation = await prompt('Please enter file location including name of private PGP Key: ');
        answer = await confirm(`Is this correct: "${this.pgpPrivateKeyLocation}"?`);
        if(!answer) console.log('No problem, let\'s try again.');
      }while(!answer)
      this.lastAction = 'initialization';

      try{
        const data = await fetch(`${this.remoteIP}/initialize`,{
          method: 'POST',
          body: JSON.stringify({key: fs.readFileSync(this.pgpPublicKeyLocation).toString()}),
          headers: {
            'Content-Type': 'application/json',
            username: this.username,
            email: this.email
          }
        })
          .then(res=> res.json())
          .then(res=>{
            if(res.success){
              fs.writeFileSync('./user.json',JSON.stringify(this));
              console.log('Initialization success!');
            }else{
              console.log(res.error);
            } //end if
          });
      }catch(err){
        console.log('Problem connecting to server.');
        console.log(err);
      }
    }
    getVersionNumber(){
      if(!fs.existsSync('./package.json')){
        console.log('Broker not installed properly, "package.json" missing.');
        process.exit(1);
      } //end if
      const {version} = JSON.parse(fs.readFileSync('./package.json'));

      return chalk.green(`Broker v${version}`);
    }
    getLoggedInRemoteIP(){
      return chalk.green('Remote Server IP: ')+chalk.white(this.remoteIP||'Not set');
    }
    getLoggedInUsername(){
      return chalk.green('Current User: ')+chalk.white(this.username||'Not logged in');
    }
    getLoggedInEmail(){
      return chalk.green('Current Email: ')+chalk.white(this.email||'Not logged in');
    }
    getLastAuthenticationDate(){
      return chalk.green('Last Authentication: ')+chalk.white(this.lastAuthentication||'Never');
    }
    getLastAction(){
      return chalk.green('Last Action: ')+chalk.white(this.lastAction||'None');
    }
    getLastScope(){
      return chalk.green('Last Scope: ')+chalk.white(this.lastScope||'None');
    }
  }
};
