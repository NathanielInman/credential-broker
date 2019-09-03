const express = require('express');
const openpgp = require('openpgp');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');


router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req,
        requestedScope = req.body.scopeName,
        hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===requestedScope);

  // short-circuit fail-first
  if(requestedScope!==name&&!hasScopeAccess){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Get Scope (${requestedScope})`)
    );
    return res.status(401).json({error: `User "${name}" does not have get all scopes permission.`});
  }else if(requestedScope===name){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get Scope (${requestedScope}-IS-SCOPE)`)
    );
    return res.status(200).json({success: req.user});
  } //end if

  try{
    const data = (await req.broker.db.getItem(`scope:${requestedScope}`)||{});

    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Get Scope (${requestedScope})`)
    );
    console.log(req.user.key);
    console.log(decodeURIComponent(req.key));
    const passphrase = '';
    const privKeyObj = (await openpgp.key.readArmored(decodeURIComponent(req.key))).keys[0];
    console.log(privKeyObj);
    await privKeyObj.decrypt(passphrase);
    const encrypted = await openpgp.encrypt({
      message: openpgp.message.fromText(JSON.stringify(data)),        // input as Message object
      publicKeys: (await openpgp.key.readArmored(`-----BEGIN PGP PUBLIC KEY BLOCK-----
        ${req.user.key}
    -----END PGP PUBLIC KEY BLOCK-----`)).keys, //for encryption
      privateKeys: [privKeyObj]                                 // for signing (optional)
    });
    console.log(encrypted);
    res.status(200).json({success: encrypted});
  }catch(err){
    res.status(500).json({error: 'Server had a problem getting scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
