const chalk = require('chalk');

module.exports = {
  helpAbandonment(){
    console.log(chalk.green('Information: ')+chalk.white('abandonment'));
    console.log(chalk.green('Brief: ')+chalk.white('There are strategies to manage data abandonment'));
    console.log(chalk.green('Description: '));
    console.log(chalk.green('There are four different scenarios or edge-cases that must be handled when it comes to transitioning access of sensitive data.'));
    console.log(chalk.green('1. Person leaves company and their user is removed, being the only edit user of some scopes.'));
    console.log(chalk.green('  - In this scenario the broker first attempts to give edit access to the oldest created user that has view access to the scope. In this scenario it emails all members who have any access to that scope and the person who revoked access of the original user what happened. This behavior can be disabled. This strategy is called ')+chalk.cyan('oldest user acquires edit access.'));
    console.log(chalk.green('  - If there are no users with view access to scopes created by deleted user, it gives edit access of the scopes to the operator who revoked access of the user and notifies all members that also have user edit access. This behavior can be disabled. This strategy is called ')+chalk.cyan('revoker acquires abandoned scope access.'));
    console.log(chalk.green('2. Person leaves company and their user can\'t be removed as there isn\'t another user with user edit access.'))
    console.log(chalk.green('  - Any user on the broker may petition a revoking of another user. This sends an email to all users and gives the potentially revoked user 24hours to cancel the revoking before it removes the user. This behavior can be disabled. This strategy is called ')+chalk.cyan('all users allowed to revoke.'));
    console.log(chalk.green('3. There is only one user in broker and they leave a company having deleted their own account first'));
    console.log(chalk.green('  - The next user to create an account will acquire all scopes as edit access, user edit access and ability to create new scopes. This behavior can be disabled. This strategy is called ')+chalk.cyan('first account gets all access.'));
    console.log(chalk.green('4. There is only one user in broker and they leave a company without deleting their own account first, or there are many users on the broker but restoration strategies for maintaining a user with edit access has been disabled.'));
    console.log(chalk.green('  - Somebody can run the ')+chalk.cyan('broker wipe')+chalk.green(' command to remove all users from the instance. All users with access to the broker will receive an email giving them 24hours to cancel the operation. This will allow somebody to create the first user and acquire access to all scopes unless first account gets all access strategy has been disabled. The ability to allow other users 24hours to cancel a wipe can be disabled and it\'s strategy is called ')+chalk.cyan('any user can cancel a wipe.'));
    console.log(chalk.green('5. Either situation 1 through 4 happened and their restoration behaviors were disabled, any user with access to the machine can output the scopes and sensitive item names but values will forever be encrypted so maybe it\'s time to rotate those keys or generate new ones. Acquiring the names still allows you to keep a map of the data lost, but by disabling the strategies above you run the risk of not being able to access the data should these situations occur.'));

  }
}
