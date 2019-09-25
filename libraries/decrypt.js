const openpgp = require('openpgp');
const fs = require('fs');

module.exports = {
  async decrypt(user,text){
    const privateKey = fs.readFileSync('./id_rsa').toString(),
          privateKeys = (await openpgp.key.readArmored(privateKey)).keys;

    await Promise.all(privateKeys.map(k=> k.decrypt(user.email)));
    const {data} = await openpgp.decrypt({
      message: await openpgp.message.readArmored(text),
      privateKeys
    });

    return data;
  }
};
