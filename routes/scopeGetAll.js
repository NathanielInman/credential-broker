const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req;

  try{

    // short-circuit fail-first
    if(!req.user.permissions.viewScopeNames){
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
      const scopes = await req.broker.db.getItem('scopes');

      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.green(` Get All Scope Names`)
      );
      if(!scopes){
        return res.status(200).json({success: []});
      }else{
        return res.status(200).json({success: scopes.map(scope=> scope.name)});
      } //end if
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error retrieving scopes.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
