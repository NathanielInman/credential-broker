const chalk = require('chalk');
const fs = require('fs');
const fetch = require('node-fetch');
const {prompt} = require('../libraries/prompt.js');

const defaultStrategies = [
  {
    name: 'Oldest user acquires edit access',
    value: true
  },
  {
    name: 'Revoker acquires abandoned scope access',
    value: true
  },
  {
    name: 'All users allowed to revoke',
    value: true
  },
  {
    name: 'First account gets access',
    value: true
  },
  {
    name: 'Any user can cancel a wipe',
    value: true
  }
];

module.exports = {
  Broker: class Broker{
    constructor({
      externalIP='',port=4000,scopeNumber=0,userNumber=0,
      lastAccess='',lastUser='', strategies=defaultStrategies
    }={}){
      this.externalIP = externalIP;
      this.port = port;
      this.scopeNumber = scopeNumber;
      this.userNumber = userNumber;
      this.lastAccess = lastAccess;
      this.lastUser = lastUser;
      this.strategies = strategies;
    }
    async askStrategies(){
      for await(let strategy of this.strategies){
        strategy.value = await prompt(`Allow strategy: "${strategy.name}"? [Y/n]`);
      } //end for
    }
    getStrategyString(index){
      return this.strategies[index].value?
        chalk.green(`${this.strategies[index].name} ✔`):
        chalk.red(`${this.strategies[index].name} ✘`);
    }
    getVersionNumber(){
      if(!fs.existsSync('./package.json')){
        console.log('Broker not installed properly, "package.json" missing.');
        process.exit(1);
      } //end if
      const {version} = JSON.parse(fs.readFileSync('./package.json'));

      return chalk.green(`Broker v${version}`);
    }
    async getAddress(){
      this.externalIP = await fetch('http://checkip.amazonaws.com/').then(res=> res.text());
      this.externalIP = this.externalIP.replace(/\r?\n|\r/g,'');
      return chalk.green('External Address: ')+chalk.white(`${this.externalIP}:${this.port}`);
    }
  }
};
