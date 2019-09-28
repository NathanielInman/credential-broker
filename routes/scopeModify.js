const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {target,scopeName,scopePublicKey} = await verify(key,req.body);

  if(!target||!scopeName||!scopePublicKey){
    log(ip,name,'Scope Modify (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${target}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      log(ip,name,`Scope Modify (${scopeName})`,true);
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      log(ip,name,`Scope Modify (${scopeName}-NO-SCOPE)`,true);
      return res.status(401).json({
        error: `Scope "${scopeName}" does not exist.`
      });
    }else{
      log(ip,name,`Scope Modify (${target}->${scopeName})`);
      targetScope.name = scopeName;
      targetScope.scopePublicKey = scopePublicKey;
      if(target!==scopeName){
        await req.broker.db.removeItem(`scope:${target}`);
        await req.broker.db.setItem(`scope:${scopeName}`);
        const users = await req.broker.db.getItem('users');

        users.forEach(async user=>{

          // alter any existing users access to the original scopename
          const scope = user.permissions.scopes.find(s=>s.name===target);

          // for any user that had access to the scope, change the scopename
          // to the new name and update that users scope name
          if(scope){
            scope.name = scopeName;
            await req.broker.db.setItem(`user:${name}`,user);
          } //end if
        });
      }else{

        // just public key updated
        await req.broker.db.setItem(`scope:${scopeName}`);
      } //end if
      res.status(200).json({success: 'Modified scope successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
