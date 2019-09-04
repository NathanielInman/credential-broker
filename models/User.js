const chalk = require('chalk');
const openpgp = require('openpgp');
const fs = require('fs');
const fetch = require('node-fetch');
const {password,prompt,confirm} = require('../libraries/prompt.js');

const defaultPermissions = {
  viewUsers: false,
  editUsers: false,
  viewScopeNames: false,
  createScopes: false,
  scopes: []
};
module.exports = {
  User: class User{
    constructor({
      remoteIP='',name='',email='',lastAuthentication=null,
      lastAction='',lastScope='',usePassword=false,
      pgpPrivateKeyLocation='',pgpPublicKeyLocation='',
      permissions=defaultPermissions
    }={}){
      this.remoteIP = remoteIP;
      this.name = name;
      this.email = email;
      this.usePassword = usePassword;
      this.pgpPrivateKeyLocation = pgpPrivateKeyLocation;
      this.pgpPublicKeyLocation = pgpPublicKeyLocation;
      this.lastAuthentication = lastAuthentication;
      this.lastAction = lastAction;
      this.lastScope = lastScope;
      this.permissions = defaultPermissions
    }
    async initialize(){
      let answer;

      do{
        this.remoteIP = await prompt(chalk.green('Broker service ip: '));
        answer = await confirm(chalk.green(`Is this correct: "${this.remoteIP}"?`));
        if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
      }while(!answer)
      do{
        this.name = await prompt(chalk.green('Please enter name: '));
        answer = await confirm(chalk.green(`Is this correct: "${this.name}"?`));
        if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
      }while(!answer)
      do{
        this.email = await prompt(chalk.green('Please enter email: '));
        answer = await confirm(chalk.green(`Is this correct: "${this.email}"?`));
        if(!answer) console.log(chalk.green('No problem, let\'s try again.'));
      }while(!answer)
      answer = await confirm(chalk.green('Establish a password? (Will be asked on every action)'))
      if(answer){
        this.usePassword = true;
        let passphrase,passphraseConfirm;

        do{
          passphrase = await password(chalk.green('Please enter password: '));
          passphraseConfirm = await password(chalk.green('Please confirm password: '));
          answer = passphrase===passphraseConfirm;
          if(!answer){
            console.log(chalk.red('Passwords didn\'t match, let\'s try again.'));
          } //end if
        }while(!answer)
        console.log(chalk.green('Generating keys...'));
        const key = await openpgp.generateKey({
          userIds: `${this.name} <${this.email}>`,
          passphrase
        });
        console.log(chalk.green('done.'));
        console.log(chalk.green('Writing public key...'));
        fs.writeFileSync('./id_rsa.pub',key.publicKeyArmored);
        console.log(chalk.green('done.'));
        console.log(chalk.green('Writing private key...'));
        fs.writeFileSync('./id_rsa',key.privateKeyArmored);
        console.log(chalk.green('done.'));
        console.log(chalk.green('Writing revocation certificate...'));
        fs.writeFileSync('./id_rsa.revocationCertificate',key.revocationCertificate);
        console.log(chalk.green('done.'));
      }else{
        this.usePassword = false;
      } //end if
      this.lastAction = 'initialization';
      this.permissions.viewUsers = await confirm(chalk.green('Request view all users permission? '));
      this.permissions.editUsers = await confirm(chalk.green('Request edit all users permission? '));
      this.permissions.viewScopeNames = await confirm(chalk.green('Request view all scopes permission? '));
      this.permissions.createScopes = await confirm(chalk.green('Request create scopes permission? '));
      do{
        answer = await prompt(chalk.green('Enter requested scopes to access separated by commas: '));
        const scopeNames = answer.split(',').map(n=> n.trim());

        for(let i=0;i<scopeNames.length;i++){
          answer = await confirm(chalk.green(`Request full access (Y) or view access to scope "${scopeNames[i]}"(n)?`));
          this.permissions.scopes.push({name: scopeNames[i],value: answer?'edit':'view'})
        } //end for
        answer = await confirm(chalk.green(`Is this correct: "${
          this.permissions.scopes.map(s=> `(${s.name}): ${s.value}`).join()
        }"?`));
        if(!answer){
          this.permissions.scopes.length = 0;
          console.log(chalk.green('No problem, let\'s try again.'));
        } //end if
      }while(!answer)

      try{
        const data = await fetch(`${this.remoteIP}/initialize`,{
          method: 'POST',
          body: JSON.stringify({...this}),
          headers: {
            'Content-Type': 'application/json',
            key: fs.readFileSync('./id_rsa.pub').toString(),
            name: this.name,
            email: this.email
          }
        })
          .then(res=> res.json())
          .then(res=>{
            if(res.success){
              fs.writeFileSync('./user.json',JSON.stringify(this));
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
      return chalk.green('Current User: ')+chalk.white(this.name||'Not logged in');
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
