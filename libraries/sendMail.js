import nodemailer from 'nodemailer'

export function sendMail({ broker = null, to = '', title = '', body = '' } = {}) {
  if (!broker || (broker && broker.emailServer === 'None')) return
  const transporter = nodemailer.createTransport(broker.emailTransport)

  transporter.sendMail({
    from: broker.emailTransport.auth.user,
    to,
    subject: `Credential Broker - ${title}`,
    text: body,
    html: body
  })
}
