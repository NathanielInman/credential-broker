const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req;

  // short-circuit fail-first
  if(!req.user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Add User (${req.body.name})`)
    );
    return res.status(401).json({error: `User "${user.name}" does not have user edit permission.`});
  } //end if

  try{
    await req.broker.db.setitem(
      `user:${req.body.name}`,
      {
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
      }
    );
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
  res.status(200).json({success: 'Added user'});
});

module.exports = {router};
