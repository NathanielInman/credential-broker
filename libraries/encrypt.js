const openpgp = require('openpgp');

module.exports = {
  async encrypt(key,text){
    const publicKeys = (await openpgp.key.readArmored(key)).keys,
          {data} = await openpgp.encrypt({
            message: openpgp.message.fromText(text),
            publicKeys
          });

    return data;
  }
};
