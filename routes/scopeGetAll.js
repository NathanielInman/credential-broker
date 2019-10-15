const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {user} = req;

  try{

    // short-circuit fail-first
    if(!user.permissions.viewScopeNames){
      req.log('Get All Scope Names (Bad Request)',true);
      return req.respond({status:401,body:{
        error: `User "${user.name}" does not have get all scope names permission.`
      }});
    }else{
      req.log('Get All Scope Names');
      const scopes = await req.broker.db.getItem('scopes'),
            scopeData = JSON.stringify({
              success: (scopes||[]).map(scope=> scope.name)
            });

      req.respond({body:scopeData});
    } //end if
  }catch(err){
    req.respond({status:500,body:{error: 'Server error retrieving scopes.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
