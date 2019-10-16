const openpgp = require('openpgp');
const fs = require('fs');
const {password} = require('./prompt.js');

module.exports = {
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
