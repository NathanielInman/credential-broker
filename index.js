const chalk = require('chalk');
const {
  help,start,
  userInitialize,userAdd,userDelete,userModify,userGetAll,userGet,
  scopeAdd,scopeDelete,scopeModify,scopeGetAll,scopeGet,
  secretAdd,secretDelete,secretModify
} = require('./commands/index.js');

// ignore the first two arguments
// argument number 1: working directory of node
// argument number 2: location of this script
const args = process.argv.slice(2);

if(!args.length){ help();
}else if(args[0]==='help'&&!args[1]){ help();
}else if(args[0]==='help'&&args[1]){ help(args.slice(1).join('-'));
}else if(args[0]==='start'){ start();
}else if(args[0]==='init'){ userInitialize();
}else if(args[0]==='user'&&!args[1]){ help('user')
}else if(args[0]==='user'&&args[1]==='add'&&!args[2]){ help('user add');
}else if(args[0]==='user'&&args[1]==='add'&&args[2]){ userAdd(args[2]);
}else if(args[0]==='user'&&args[1]==='del'&&!args[2]){ help('user del');
}else if(args[0]==='user'&&args[1]==='del'&&args[2]){ userDelete(args[2]);
}else if(args[0]==='user'&&args[1]==='mod'&&!args[2]){ help('user mod');
}else if(args[0]==='user'&&args[1]==='mod'&&args[2]){ userModify(args[2]);
}else if(args[0]==='user'&&args[1]==='get'&&!args[2]){ userGetAll();
}else if(args[0]==='user'&&args[1]==='get'&&args[2]){ userGet(args[2]);
}else if(args[0]==='scope'&&!args[1]){ help('scope');
}else if(args[0]==='scope'&&args[1]==='add'&&!args[2]){ help('scope add');
}else if(args[0]==='scope'&&args[1]==='add'&&args[2]){ scopeAdd(args[2]);
}else if(args[0]==='scope'&&args[1]==='del'&&!args[2]){ help('scope del');
}else if(args[0]==='scope'&&args[1]==='del'&&args[2]){ scopeDelete(args[2]);
}else if(args[0]==='scope'&&args[1]==='mod'&&!args[2]){ help('scope mod');
}else if(args[0]==='scope'&&args[1]==='mod'&&args[2]){ scopeModify(args[2]);
}else if(args[0]==='scope'&&args[1]==='get'&&!args[2]){ scopeGetAll();
}else if(args[0]==='scope'&&args[1]==='get'&&args[2]){ scopeGet(args[2]);
}else if(args[0]==='get'&&!args[1]){ scopeGetAll();
}else if(args[0]==='get'&&args[1]){ scopeGet(args[1]);
}else if(args[0]==='add'&&!args[1]){ help('add');
}else if(args[0]==='add'&&args[1]&&!args[2]){ help('add');
}else if(args[0]==='add'&&args[1]&&args[2]){ secretAdd(args[1],args[2]);
}else if(args[0]==='del'&&!args[1]){ help('del');
}else if(args[0]==='del'&&args[1]&&!args[2]){ help('del');
}else if(args[0]==='del'&&args[1]&&args[2]){ secretDelete(args[1],args[2]);
}else if(args[0]==='mod'&&!args[1]){ help('mod');
}else if(args[0]==='mod'&&!args[1]&&!args[2]){ help('mod');
}else if(args[0]==='mod'&&args[1]&&args[2]){ secretModify(args[1],args[2]);
}else{ help('invalid');
} //end if
