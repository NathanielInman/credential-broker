const fs = require('fs');
const chalk = require('chalk');
const bodyParser = require('body-parser');
const {Broker} = require('../models/Broker.js');
const {router} = require('../routes/index.js');

module.exports = {
  async start(){
    let broker;

    if(!fs.existsSync('./broker.json')){
      broker = new Broker;

      await broker.askStrategies();
      fs.writeFileSync('./broker.json',JSON.stringify(broker));
    }else{
      broker = new Broker(JSON.parse(fs.readFileSync('./broker.json')));
    } //end if
    const express = require('express');
    const app = express();

    app.use(express.json());
    app.use((req,res,next)=>{
      req.ip = req.headers['x-forwarded-for'] || req.connection.remoteAddress;
      req.name = req.headers.name||'';
      req.email = req.headers.email||'';
      next();
    });
    app.use(async (req,res,next)=>{
      await broker.dbLoading;
      req.broker = broker;
      next();
    });
    app.use(router);
    console.log(chalk.blue( '         ./*.           '));
    console.log(chalk.blue( '     /@@@@@@@@@@.       '));
    console.log(chalk.blue( '   ,@@@@,    /@@@@      '));
    console.log(chalk.blue( '  .@@@.        *@@@     '));
    console.log(chalk.blue( '  @@@,          &@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@, ')+broker.getVersionNumber());
    console.log(chalk.blue( '@@@@@@@@@@%%@@@@@@@@@@, ')+await broker.getAddress());
    console.log(chalk.blue( '@@@@@@@@#    ,@@@@@@@@, ')+broker.getStrategyString(0));
    console.log(chalk.blue( '@@@@@@@@&    #@@@@@@@@, ')+broker.getStrategyString(1));
    console.log(chalk.blue( '@@@@@@@@@.   @@@@@@@@@, ')+broker.getStrategyString(2));
    console.log(chalk.blue( '@@@@@@@@@@#(@@@@@@@@@@, ')+broker.getStrategyString(3));
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@  ')+broker.getStrategyString(4));
    app.listen(broker.port);
  }
};
