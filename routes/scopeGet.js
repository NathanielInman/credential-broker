const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        requestedScope = req.body.name,
        hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===requestedScope);

  // short-circuit fail-first
  if(requestedScope!==name&&!hasScopeAccess){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Get Scope (${requestedScope})`)
    );
    return res.status(401).json({error: `User "${name}" does not have get all scopes permission.`});
  }else if(requestedScope===name){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get Scope (${requestedScope}-IS-SCOPE)`)
    );
    return res.status(200).json({success: req.user});
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get Scope (${requestedScope})`)
    );
    res.status(200).json({
      success: await req.broker.db.getItem(`scope:${requestedScope}`)
    });
  }catch(err){
    res.status(500).json({error: 'Server had a problem getting scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
