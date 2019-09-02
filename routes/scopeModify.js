const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        targetScopeOriginalName = req.body.target,
        targetScopename = req.body.scopeName,
        hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===targetScopename),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${targetScopeOriginalName}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Scope Modify (${targetScopename})`)
      );
      return res.status(401).json({
        error: `User "${user.name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Scope Modify (${targetScopename}-NO-SCOPE)`)
      );
      return res.status(401).json({
        error: `Scope "${targetScopename}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.green(`Scope Modify (${targetScopeOriginalName}->${targetScopename})`)
      );
      targetScope.name = targetScopename;
      targetScope.scopePublicKey = req.body.scopePublicKey;
      if(targetScopeOriginalName!==targetScopename){
        await req.broker.db.removeItem(`scope:${targetScopeOriginalName}`);
        await req.broker.db.setItem(`scope:${targetScopename}`);
        const users = await req.broker.db.getItem('users');

        users.forEach(user=>{

          // alter any existing users access to the original scopename
          const scope = user.permissions.scopes.find(s=>s.name===targetScopeOriginalName);

          // for any user that had access to the scope, change the scopename
          // to the new name and update that users scope name
          if(scope){
            scope.name = targetScopename;
            await req.broker.db.setItem(`user:${user.name}`,user);
          } //end if
        });
      }else{

        // just public key updated
        await req.broker.db.setItem(`scope:${targetScopename}`);
      } //end if
      res.status(200).json({success: 'Modified scope successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
