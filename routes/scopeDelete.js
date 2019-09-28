const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName} = await verify(key,req.body);

  //short-circuit fail-first
  if(!scopeName){
    log(ip,name,'Scope Delete (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      log(ip,name,`Scope Delete (${scopeName})`,true);
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      log(ip,name,`Scope Delete (${scopeName}-NO_SCOPE)`,true);
      return res.status(401).json({error: `Scope "${scopeName}" does not exist.`});
    }else{
      log(ip,name,`Scope Delete (${scopeName})`);
      const users = await req.broker.db.getItem('users');

      users.forEach(user=>{

        // remove any existance of the deleted scope throughout all users
        user.permissions.scopes = user.permissions.scopes.filter(s=>s.name!==scopeName);
      });

      await req.broker.db.setItem('users',users);
      await req.broker.db.removeItem(`scope:${scopeName}`);
      const scopes = await req.broker.db.getItem('scopes');

      await req.broker.db.setItem('scopes',scopes.filter(s=> s.name!==scopeName));
      res.status(200).json({success: 'Deleted scope successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
