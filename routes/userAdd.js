const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name,user,key} = req;

  // short-circuit fail-first
  if(!user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Add User (${req.body.name})`)
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
      chalk.green(` Add User (${req.body.name})`)
    );
    let newUser = {
      date: (new Date).toISOString(),
      name: req.body.name,
      email: req.body.email,
      addedBy: name,
      addedByIP: ip,
      key: req.body.key,
      permissions: {
        viewUsers: req.body.permissions.viewUsers,
        editUsers: req.body.permissions.editUsers,
        viewScopeNames: req.body.permissions.viewScopeNames,
        createScopes: req.body.permissions.createScopes,
        scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
      }
    };

    const users = await req.broker.db.getItem('users');

    await req.broker.db.setItem(`user:${req.body.name}`,newUser);
    users.push(newUser);
    await req.broker.db.setItem('users',users);
    res.status(200).json({success: `Added user ${newUser.name}`});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
