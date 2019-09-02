const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        requestedUsername = req.body.name;

  try{

    // short-circuit fail-first
    if(!req.user.permissions.viewUsers){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` Get All Users`)
      );
      return res.status(401).json({error: `User "${user.name}" does not have user edit permission.`});
    }else{
      const users = await req.broker.db.getItem('users');

      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.green(` Get All Users`)
      );
      if(!users){
        return res.status(200).json({success: []});
      }else{
        return res.status(200).json({success: users.map(user=> user.name)});
      } //end if
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error retrieving users.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
