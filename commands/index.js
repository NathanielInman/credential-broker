const {help} = require('./help.js');
const {start} = require('./start.js');
const {init} = require('./init.js');
const {userAdd} = require('./userAdd.js');
const {userDelete} = require('./userDelete.js');
const {userModify} = require('./userModify.js');
const {userGetAll} = require('./userGetAll.js');
const {userGet} = require('./userGet.js');
const {scopeAdd} = require('./scopeAdd.js');
const {scopeDelete} = require('./scopeDelete.js');
const {scopeModify} = require('./scopeModify.js');
const {scopeGetAll} = require('./scopeGetAll.js');
const {scopeGet} = require('./scopeGet.js');
const {secretAdd} = require('./secretAdd.js');
const {secretDelete} = require('./secretDelete.js');
const {secretModify} = require('./secretModify.js');

module.exports = {
  help,start,init,
  userAdd,userDelete,userModify,userGetAll,userGet,
  scopeAdd,scopeDelete,scopeModify,scopeGetAll,scopeGet,
  secretAdd,secretDelete,secretModify
};
