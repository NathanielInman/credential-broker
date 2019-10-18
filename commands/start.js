const fs = require('fs');
const chalk = require('chalk');
const bodyParser = require('body-parser');
const {Broker} = require('../models/Broker.js');
const {log} = require('../libraries/log.js');
const {respond} = require('../libraries/respond.js');
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

    app.use(async (req,res,next)=>{
      req.ip = req.headers['x-forwarded-for'] || req.connection.remoteAddress;
      req.key = decodeURIComponent(req.headers.key||'');
      req.log = (string,isError)=> log(req.ip,req.name,string,isError);
      req.respond = ({status=200,body=''})=> respond({req,res,status,body});

      // ensure the broker settings are loaded, they can be used for configuration
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
    console.log(broker.getSessionTTL());
    console.log(broker.getTwoFactorTTL());
    console.log(broker.getServerEmail());
    app.listen(broker.port);
  }
};
