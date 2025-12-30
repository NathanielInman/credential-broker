import chalk from 'chalk'

export function helpInit() {
  console.log(chalk.green('Command: ') + chalk.white('init'))
  console.log(
    chalk.green('Brief: ') + chalk.white('Initialize the local account before using broker')
  )
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      '  Before using the command line broker client for accessing the server, the user must first create their local account. It starts by asking for the remote address of the broker server. It then asks for the operators username. It then asks for the location of the operators private PGP key so ti can automate the logging in process for the user. It attempts to authenticate with the specified server to see if the operators username exists. If the server connects and the account exists as a user then it completes successfully.'
    )
  )
  console.log(
    chalk.white(
      "  If the server connects and there are no existing accounts, it gives the operator edit access to all existing scopes and scope create access & user edit access unless that restoration strategy has been disabled. Keep in mind that just because they inherit all existing data, they won't be able to view or edit any secrets or scope that are created by others after this point unless those scopes become abandoned (provided this restoration strategy hasnt been turned off.) The purpose of this structure is to allow admins to inherit data that was abandoned by previous users, such as when amployees leave a company. To read more about these sensitive cases and ways of managing risk and configuration see the help file: "
    ) + chalk.cyan('broker help abandonment')
  )
  console.log(
    chalk.white(
      "  If the server connects and there are existing accounts, but the operators account doesn't exist, it informs the operator to contact the broker admins and then lists all usernames on the service that have access to editing users."
    )
  )
}
