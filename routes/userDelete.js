const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        targetUsername = req.body.target;

  try{
    const targetUser = await req.broker.db.getItem(`user:${targetUsername}`);

    // short-circuit fail-first
    if(!user.permissions.editUsers){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` User Delete (${targetUsername})`)
      );
      return res.status(401).json({
        error: `User "${targetUsername}" does not have user edit permission.`
      });
    }else if(!targetUser){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` User Delete (${targetUsername}-NO-USER)`)
      );
      return res.status(401).json({
        error: `User "${targetUsername}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.green(` User Delete (${targetUsername})`)
      );
      const users = await req.broker.db.getItem('users');

      await req.broker.db.setItem('users',users.filter(u=>u.name!==targetUsername));
      await req.broker.db.removeItem(`user:${targetUsername}`);
      res.status(200).json({success: 'Deleted user successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
