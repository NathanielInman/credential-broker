const nodemailer = require('nodemailer');
const chalk = require('chalk');

const transporter = nodemailer.createTransport({
  sendmail: true,
  newline: 'unix',
  path: '/usr/sbin/sendmail'
});

module.exports = {
  mail({
    from='unknown@credential.broker',to='unknown@credential.broker',
    subject='Unknown Subject',text='Unknown Text'
  }={}){
    console.log('node mailer stuff', JSON.stringify({from,to,subject,text}));
    transporter.sendMail({from,to,subject,text},err=>{
      console.log(JSON.stringify(arguments));
      if(err) console.log(chalk.red(err));
    });
  }
};
