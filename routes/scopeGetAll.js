const express = require('express');
const openpgp = require('openpgp');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name,key,user,passphrase} = req;

  try{

    // short-circuit fail-first
    if(!user.permissions.viewScopeNames){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` Get All Scope Names`)
      );
      return res.status(401).json({
        error: `User "${user.name}" does not have get all scope names permission.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.green(` Get All Scope Names`)
      );
      const scopes = await req.broker.db.getItem('scopes'),
            scopeData = JSON.stringify(!scopes?[]:scopes.map(scope=> scope.name)),
            keyPublicObj = await openpgp.key.readArmored(user.key),
            {data} = await openpgp.encrypt({
              message: openpgp.message.fromText(scopeData),
              publicKeys: (keyPublicObj).keys
            });

      res.status(200).json({success: data});
    } //end if
  }catch(err){
    if(err==='Error: Incorrect key passphrase'){
      res.status(401).json({error: 'Incorrect key passphrase.'});
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.red(` Incorrect key passphrase`)
      );
    }else{
      res.status(500).json({error: 'Server error retrieving scopes.'})
      console.log(chalk.red(err));
    } //end if
  }
});

module.exports = {router};
