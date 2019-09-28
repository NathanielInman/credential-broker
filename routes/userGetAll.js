const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {encrypt} = require('../libraries/encrypt.js');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        requestedUsername = await verify(key,req.body);

  try{

    // short-circuit fail-first
    if(!requestedUsername){
      log(ip,name,'Get All Users (SIGNING-VERIFICATION-FAILURE)',true);
      return res.status(403).json({error: 'Request has been tempered with!'});
    }else if(!user.permissions.viewUsers){
      log(ip,name,'Get All Users',true);
      return res.status(401).json({error: `User "${name}" does not have user edit permission.`});
    }else{
      log(ip,name,'Get All Users');
      const users = await req.broker.db.getItem('users'),
            userData = JSON.stringify(!users?[]:users.map(user=> user.name));

      res.status(200).json({success: await encrypt(key,userData)})
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error retrieving users.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
