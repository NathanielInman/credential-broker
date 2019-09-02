const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        targetScopename = req.body.target,
        hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===targetScopename),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${targetScopename}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Scope Delete (${targetScopename})`)
      );
      return res.status(401).json({
        error: `User "${req.user.name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Scope Delete (${targetScopename}-NO-SCOPE)`)
      );
      return res.status(401).json({
        error: `Scope "${targetScopename}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.green(`Scope Delete (${targetScopename})`)
      );
      const users = await req.broker.db.getItem('users');

      users.forEach(user=>{

        // remove any existance of the deleted scope throughout all users
        user.permissions.scopes = user.permissions.scopes.filter(s=>s.name!==targetScopename);
      });

      await req.broker.db.setItem('users',users);
      await req.broker.db.removeItem(`scope:${targetScopename}`);
      const scopes = await req.broker.db.getItem('scopes');

      await req.broker.db.setItem('scopes',scopes.filter(s=> s.name!==targetScopename));
      res.status(200).json({success: 'Deleted scope successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};