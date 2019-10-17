const openpgp = require('openpgp');
const chalk = require('chalk');
const fs = require('fs');
const {password} = require('./prompt.js');

module.exports = {
  async decrypt(user,text){
    const privateKey = fs.readFileSync('./id_rsa').toString(),
          privateKeys = (await openpgp.key.readArmored(privateKey)).keys,
          passwordValue = !user.usePassword?user.email:
            await password(chalk.green('Please enter password: '));

    await Promise.all(privateKeys.map(k=> k.decrypt(passwordValue)));
    const {data} = await openpgp.decrypt({
      message: await openpgp.message.readArmored(text),
      privateKeys
    });

    return data;
  },
  async encrypt(key,text){
    const publicKey = fs.readFileSync('./id_rsa.pub','utf8');

    const publicKeys = (await openpgp.key.readArmored(publicKey)).keys,
          {data} = await openpgp.encrypt({
            message: openpgp.message.fromText(text),
            publicKeys
          });

    return data;
  },
  async sign(user,text){
    const privateKey = fs.readFileSync('./id_rsa').toString(),
          privateKeys = (await openpgp.key.readArmored(privateKey)).keys,
          passwordValue = !user.usePassword?user.email:
            await password(chalk.green('Please enter password: '));

    await Promise.all(privateKeys.map(k=> k.decrypt(passwordValue)));
    const {data} = await openpgp.sign({
      message: openpgp.cleartext.fromText(text),
      privateKeys
    });

    return data;
  }
};
