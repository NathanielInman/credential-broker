const express = require('express');
const chalk = require('chalk');

module.exports = {
  async authenticate(req,res,next){
    const {ip,name} = req;

    // short-circuit failure
    if(!name){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<unknown>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`${req.originalUrl}: Username missing`)
      );
      return res.status(401).json({error: 'Your name is not passed in the header.'});
    } //end if

    const user = await req.broker.db.getItem(`user:${name}`);

    //short-circuit failure
    if(!user){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`${req.originalUrl}: User does not exist`)
      );
      return res.status(401).json({error: 'Your user does not exist.'});
    } //end if

    //short-circuit failure
    if(!user.permissions){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`${req.originalUrl}: User data is corrupted`)
      );
      return res.status(401).json({error: 'Your user structure invalid, data corrupted.'});
    } //end if

    // attach the user to the request so any route can attach their
    // individual requirements or filters based on their permissions
    req.user = user;
    next();
  }
};
