const express = require('express');
const router = express.Router({mergeParams: true});
const chalk = require('chalk');

router.post('/initialize',(req,res)=>{
  const ip = req.headers['x-forwarded-for'] ||
          req.connection.remoteAddress,
        username = req.headers.username;

  console.log(req.body);
  console.log(
    chalk.cyan(`[${ip}]`)+
    chalk.magenta(`<${username}>`)+
    chalk.grey(':')+
    chalk.green(' Get Root')
  );
  res.status(200).json({success: 'User exists and is validated'});
  /*
# If the server connects and the account exists as a user it completes successful
# If the server connects and there are no existing accounts,
#  it gives the operator edit access to all existing scopes and scope create access & user
#  edit access unless this restoration strategy has been disabled. keep in mind that
#  just because they inherit all existing data, they won't be able to view or
#  edit any secrets or scopes that are created by others after this point unless those scopes
#  become abandoned. The purpose of this structure is to allow admins to inherit data that was 
#  abandoned by previous users, such as when employees leave a company. To read more about
#  these sensitive cases, refer to the ABANDONMENT section in the readme
# If the server connects and there are existing accounts, but the
#  operators account doesn't exist it informs the operator to contact the broker
#  admins and then lists the users that have user edit access*/
});

module.exports = {router};
