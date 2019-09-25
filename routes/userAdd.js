const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        newUser = await verify(key,req.body);

  if(!newUser){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`User Add (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  } //end if

  // short-circuit fail-first
  if(!user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Add User (${newUser.name})`)
    );
    return res.status(401).json({
      error: `User "${name}" does not have user edit permission.`
    });
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Add User (${newUser.name})`)
    );
    let newUser = {
      date: (new Date).toISOString(),
      name: newUser.name,
      email: newUser.email,
      addedBy: name,
      addedByIP: ip,
      key: newUser.key,
      permissions: {
        viewUsers: newUser.permissions.viewUsers,
        editUsers: newUser.permissions.editUsers,
        viewScopeNames: newUser.permissions.viewScopeNames,
        createScopes: newUser.permissions.createScopes,
        scopes: newUser.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
      }
    };

    const users = await req.broker.db.getItem('users');

    await req.broker.db.setItem(`user:${newUser.name}`,newUser);
    users.push(newUser);
    await req.broker.db.setItem('users',users);
    res.status(200).json({success: `Added user ${newUser.name}`});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
