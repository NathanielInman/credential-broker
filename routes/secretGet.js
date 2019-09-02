const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        targetScopename = req.body.scopeName,
        hasScopeViewAccess = req.user.permissions.scopes.find(s=> s.name===targetScopename),

  try{
    const targetScope = await req.broker.db.getItem(`scope:${targetScopename}`);

    // short-circuit fail-first
    if(!hasScopeViewAccess){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Get (${targetScopename})`)
      );
      return res.status(401).json({
        error: `User "${req.user.name}" does not have scope view permission.`
      });
    }else if(!targetScope){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Get (${targetScopename}-NO-SCOPE)`)
      );
      return res.status(401).json({
        error: `Scope "${targetScopename}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.green(`Secret Get (${targetScopename})`)
      );
      res.status(200).json(targetScope);
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
