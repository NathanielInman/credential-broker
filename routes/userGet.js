const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        requestedUsername = req.body.name;

  // short-circuit fail-first
  if(requestedUsername!==name||!req.user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Get User (${requestedUsername})`)
    );
    return res.status(401).json({error: `User "${user.name}" does not have user edit permission.`});
  }else if(requestedUsername===name){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get User (${requestedUsername})`)
    );
    return res.status(200).json({success: req.user});
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get User (${requestedUsername})`)
    );
    res.status(200).json({
      success: await req.broker.db.getItem(`user:${requestedUsername}`)
    });
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
