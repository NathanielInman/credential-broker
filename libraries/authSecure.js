const fs = require('fs');
const crypto = require('crypto');
const fetch = require('node-fetch');

module.exports = {
  async authSecure(user){
    const diffieHellman = crypto.createDiffieHellman(512),
          prime = diffieHellman.getPrime('base64'),
          client = crypto.createDiffieHellman(prime,'base64'),
          key = client.generateKeys('base64'),
          id = crypto.createHash('md5').update(user.name).digest('hex'),
          serverKey = await fetch(`${user.remoteIP}/authSecure`,{
            method: 'POST',
            headers: {id,key,prime}
          }).then(res=> res.json()),
          secret = client.computeSecret(serverKey,'base64','base64');

    user.secret = secret;
    fs.writeFileSync('./user.json',JSON.stringify(user));
  },
  authSecureEncrypt(secret,content){
    const iv = crypto.randomBytes(16),
          cipher = crypto.createCipheriv('aes-256-cbc',secret.substr(0,32),iv),
          encrypted = cipher.update(content,'utf8','base64')+cipher.final('base64');

    return encodeURIComponent(`${iv.toString('hex')}:${encrypted}`);
  },
  authSecureDecrypt(secret,content){
    const parts = decodeURIComponent(content).split(':'),
          iv = Buffer.from(parts.shift(),'hex'),
          decipher = crypto.createDecipheriv('aes-256-cbc',secret.substr(0,32),iv),
          decrypted = decipher.update(parts.shift(),'base64','utf8')+decipher.final('utf8');

    return decrypted;
  }
};
