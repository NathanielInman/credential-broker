const chalk = require('chalk');
const fs = require('fs');

module.exports = {
  BrokerState: class BrokerState{
    constructor({
      remoteIP=null,username='',email='',lastAuthentication=null,
      lastAction='',lastScope=''
    }={}){
      this.remoteIP = remoteIP;
      this.username = username;
      this.email = email;
      this.lastAuthentication = lastAuthentication;
      this.lastAction = lastAction;
      this.lastScope = lastScope;
    }
    getVersionNumber(){
      if(!fs.existsSync('./package.json')){
        console.log('Broker not installed properly, "state.json" missing.');
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
