import * as openpgp from 'openpgp'
import chalk from 'chalk'
import fs from 'fs'
import { password } from './prompt.js'

export async function decrypt(user, text) {
  const privateKeyArmored = fs.readFileSync('./id_rsa').toString()
  const passwordValue = !user.usePassword
    ? user.email
    : await password(chalk.green('Please enter password: '))

  const privateKey = await openpgp.decryptKey({
    privateKey: await openpgp.readPrivateKey({ armoredKey: privateKeyArmored }),
    passphrase: passwordValue
  })

  const message = await openpgp.readMessage({ armoredMessage: text })

  const { data } = await openpgp.decrypt({
    message,
    decryptionKeys: privateKey
  })

  return data
}

export async function encrypt(key, text) {
  const publicKeyArmored = fs.readFileSync('./id_rsa.pub', 'utf8')
  const publicKey = await openpgp.readKey({ armoredKey: publicKeyArmored })

  const encrypted = await openpgp.encrypt({
    message: await openpgp.createMessage({ text }),
    encryptionKeys: publicKey
  })

  return encrypted
}

export async function sign(user, text) {
  const privateKeyArmored = fs.readFileSync('./id_rsa').toString()
  const passwordValue = !user.usePassword
    ? user.email
    : await password(chalk.green('Please enter password: '))

  const privateKey = await openpgp.decryptKey({
    privateKey: await openpgp.readPrivateKey({ armoredKey: privateKeyArmored }),
    passphrase: passwordValue
  })

  const message = await openpgp.createCleartextMessage({ text })

  const signedMessage = await openpgp.sign({
    message,
    signingKeys: privateKey
  })

  return signedMessage
}
