const chalk = require('chalk');
const fs = require('fs');

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
      permissions=defaultPermissions,secret=''
    }={}){
      this.remoteIP = remoteIP;
      this.name = name;
      this.email = email;
      this.usePassword = usePassword;
      this.lastAuthentication = lastAuthentication;
      this.lastAction = lastAction;
      this.lastScope = lastScope;
      this.permissions = defaultPermissions;
      this.secret = secret;
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
