const keypress = require('keypress');

function prompt(msg){
  return (new Promise((res) => {
    process.stdout.write(msg);
    process.stdin.setEncoding('utf8');
    process.stdin.once('data', (val) => {
      res(val.trim());
    }).resume();
  })).then((res) => {
    process.stdin.pause();
    return res;
  }).catch((e) => {
    process.stdin.pause();
    return Promise.reject(e);
  });
} //end prompt()

module.exports = {
  prompt,
  multiline(msg){
    return (new Promise((res) => {
      const buf = [];
      console.log(msg);
      process.stdin.setEncoding('utf8');
      process.stdin.on('data', (val) => {
        if ('\n' === val || '\r\n' === val) {
          process.stdin.removeAllListeners('data');
          res(buf.join('\n'));
        } else {
          buf.push(val.trimRight());
        }
      }).resume();
    })).then((res) => {
      process.stdin.pause();
      return res;
    }).catch((e) => {
      process.stdin.pause();
      return Promise.reject(e);
    });
  },
  confirm(msg){
    return prompt(`${msg} [Y/n]`).then(val=>{
      return !val.length||/^y|ye|yes|ok|true$/i.test(val.toLowerCase());
    });
  },
  password(msg,mask='*'){
    const isTTY = process.stdin.isTTY;

    if(!isTTY) return prompt(msg);

    return (new Promise(rec=>{
      let buf = '';

      keypress(process.stdin);

      process.stdin.setRawMode(true);
      process.stdout.write(msg);

      process.stdin.on('keypress', (c, key) => {
        if (key && 'return' === key.name) {
          console.log();
          process.stdin.pause();
          process.stdin.removeAllListeners('keypress');
          process.stdin.setRawMode(false);
          if (!buf.trim().length) {
            return exports.password(msg, mask)(rec);
          }
          rec(buf);
          return;
        }

        if (key && key.ctrl && 'c' === key.name) {
          console.log('%s', buf);
          process.exit();
        }

        process.stdout.write(mask);
        buf += c;
      }).resume();
    })).then(res=> {
      process.stdin.pause();
      return res;
    }).catch(e=> {
      process.stdin.pause();
      return Promise.reject(e);
    });
  }
};
