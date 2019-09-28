const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {target} = await verify(key,req.body);

  if(!target){
    log(ip,name,'User Delete (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  try{
    const targetUser = await req.broker.db.getItem(`user:${target}`);

    // short-circuit fail-first
    if(!user.permissions.editUsers){
      log(ip,name,`User Delete (${target})`,true);
      return res.status(401).json({
        error: `User "${target}" does not have user edit permission.`
      });
    }else if(!targetUser){
      log(ip,name,`User Delete (${target}-NO-USER)`,true);
      return res.status(401).json({error: `User "${target}" does not exist.`});
    }else{
      log(ip,name,`User Delete (${target})`);
      const users = await req.broker.db.getItem('users');

      await req.broker.db.setItem('users',users.filter(u=>u.name!==target));
      await req.broker.db.removeItem(`user:${target}`);
      res.status(200).json({success: 'Deleted user successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
