const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        requestedUsername = req.body.name;

  try{
    const user = await req.broker.db.getItem(`user:${requestedUsername}`);

    // short-circuit fail-first
    if(!req.user.permissions.editUsers){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` User Modify (${req.body.name})`)
      );
      return res.status(401).json({
        error: `User "${user.name}" does not have user edit permission.`
      });
    }else if(!user){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(` User Modify (${req.body.name}-NO-USER)`)
      );
      return res.status(401).json({
        error: `User "${user.name}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(':')+
        chalk.green(` User Modify (${req.body.name})`)
      );
      await req.broker.db.setItem(
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
      res.status(200).json({success: 'Modified user successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
