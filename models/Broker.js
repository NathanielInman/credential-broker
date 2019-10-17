const chalk = require('chalk');
const fs = require('fs');
const fetch = require('node-fetch');
const {prompt,confirm} = require('../libraries/prompt.js');
const storage = require('node-persist');

const defaultSessionTTL = 1000*60*5; //5 minutes
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

function prettyPrintMS(ms){
  const seconds = Math.floor(ms/1000),
        minutes = Math.floor(seconds/60),
        hours = Math.floor(minutes/60),
        days = Math.floor(hours/24),
        weeks = Math.floor(days/7);

  let result = [];

  if(weeks) result.push(`${weeks} week${weeks>1?'s':''}`);
  if(days) result.push(`${days%7} day${days>1?'s':''}`);
  if(hours) result.push(`${hours%24} hour${hours>1?'s':''}`);
  if(minutes) result.push(`${minutes%60} minute${minutes>1?'s':''}`);
  if(seconds) result.push(`${seconds%60} second${seconds>1?'s':''}`);
  result.push(`${ms%1000} millisecond${ms>1?'s':''}`);
  return result.join(' ');
} //end prettyPrintMS()

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
      this.sessionTTL = defaultSessionTTL;
      this.db = storage;
      this.dbLoading = storage.init({
        dir: './data/',
        logging: false,
        ttl: false
      });
    }
    async askStrategies(){
      for await(let strategy of this.strategies){
        strategy.value = await confirm(chalk.green(`Allow strategy: "${strategy.name}"?`));
      } //end for
      let answer;

      answer = await confirm(chalk.green(`Change session TTL? (${prettyPrintMS(this.sessionTTL)})`));
      if(answer){
        do{
          this.sessionTTL = parseInt(await prompt(chalk.green('Enter session TTL(in ms): ')));
          if(isNaN(this.sessionTTL)) this.sessionTTL = defaultSessionTTL;
          answer = await confirm(chalk.green(`Is this correct: "${prettyPrintMS(this.sessionTTL)}"`));
          if(!answer) console.log('No problem, let\'s try again.');
        }while(!answer)
      } //end if
    }
    getStrategyString(index){
      return this.strategies[index].value?
        chalk.green(`${this.strategies[index].name} ✔`):
        chalk.red(`${this.strategies[index].name} ✘`);
    }
    getSessionTTL(){
      return chalk.green(`Session TTL: ${prettyPrintMS(this.sessionTTL)}`);
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
