const chalk = require('chalk');
const {
  help,start,init,
  userAdd,userDelete,userModify,userGetAll,userGet,
  scopeAdd,scopeDelete,scopeModify,scopeGetAll,scopeGet,
  secretAdd,secretDelete,secretModify
} = require('./commands/index.js');

// ignore the first two arguments
// argument number 1: working directory of node
// argument number 2: location of this script
const args = process.argv.slice(2);

console.log('---');
console.log(args);
console.log('---');
if(!args.length){ help();
}else if(args[0]==='help'){ help();
}else if(args[0]==='start'){ start();
}else if(args[0]==='init'){ init();
}else if(args[0]==='user'&&!args[1]){ help('user')
}else if(args[0]==='user'&&args[1]==='add'&&!args[2]){ help('user add');
}else if(args[0]==='user'&&args[1]==='add'&&args[2]){ userAdd();
}else if(args[0]==='user'&&args[1]==='del'&&!args[2]){ help('user del');
}else if(args[0]==='user'&&args[1]==='del'&&args[2]){ userDelete();
}else if(args[0]==='user'&&args[1]==='mod'&&!args[2]){ help('user mod');
}else if(args[0]==='user'&&args[1]==='mod'&&args[2]){ userModify();
}else if(args[0]==='user'&&args[1]==='get'&&!args[2]){ userGetAll();
}else if(args[0]==='user'&&args[1]==='get'&&args[2]){ userGet();
}else if(args[0]==='scope'&&!args[1]){ help('scope');
}else if(args[0]==='scope'&&args[1]==='add'&&!args[2]){ help('scope add');
}else if(args[0]==='scope'&&args[1]==='add'&&args[2]){ scopeAdd();
}else if(args[0]==='scope'&&args[1]==='del'&&!args[2]){ help('scope del');
}else if(args[0]==='scope'&&args[1]==='del'&&args[2]){ scopeDelete();
}else if(args[0]==='scope'&&args[1]==='mod'&&!args[2]){ help('scope mod');
}else if(args[0]==='scope'&&args[1]==='mod'&&args[2]){ scopeModify();
}else if(args[0]==='scope'&&args[1]==='get'&&!args[2]){ scopeGetAll();
}else if(args[0]==='scope'&&args[1]==='get'&&args[2]){ scopeGet();
}else if(args[0]==='get'&&!args[1]){ scopeGet();
}else if(args[0]==='get'&&args[1]){ secretGet();
}else if(args[0]==='add'&&!args[1]){ help('add');
}else if(args[0]==='add'&&args[1]){ secretAdd();
}else if(args[0]==='del'&&!args[1]){ help('del');
}else if(args[0]==='del'&&args[1]){ secretDelete();
}else if(args[0]==='mod'&&!args[1]){ help('mod');
}else if(args[0]==='mod'&&args[1]){ secretModify();
}else{ help('invalid');
} //end if
