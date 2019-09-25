const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate.js');
const {encrypt} = require('../libraries/encrypt.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user} = req;

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
            scopeData = JSON.stringify(!scopes?[]:scopes.map(scope=> scope.name));

      res.status(200).json({success: await encrypt(key,scopeData)});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error retrieving scopes.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
