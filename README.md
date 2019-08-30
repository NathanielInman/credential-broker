# Credential Broker
A secure and easy way to manage adding, deleting and retrieving sensitive information for users and applications.

## Example Usage

```bash
# Asks for secret value, hiding the input & then establishes the secret on the specified scope
broker add $scopeName $secretName

# Deletes a secret from specified scope
broker del $scopeName $secretName

# Asks for secret value, hiding the input & then changes a secret on the specified scope
broker mod $scopeName $secretName

# Establish all environment variables based on scope if authentication is successful
broker get $scopeName
```
An extensive list of commands are found [here](#commands).

## Benefits & Reasoning
  - Devops shouldn't implicitly have access to all secrets 
    - Just because an employee has access to the infrastructure doesn't mean they have a need-to-know. To view access types, you can jump to [the types of access section](#types-of-access)
  - You can attach **TTL**'s to secrets if it's mission critical from them to disappear after a certain point
  - Just because a user has access to one application doesn't mean they have access to all it's data
    - Data can be scoped to a **need-to-know**
  - No knowledge of a language or configuration structure is required, just a **few intuitive commands**
  - Adding, removing and changing secrets should be **faster** than unvaulting & editing playbooks
  - The responsibility of adding and removing secrets can be managed by organizational structure
    - Just because a developer has access to the secrets to run an application doesn't mean they have access to changing, adding or removing additional secrets
  - The system can be used to store non-sensitive information
  - There are default strategies in place to ever **prevent data loss** or access issues when employees leave.
    - For more information on the strategies you can jump to [the abandonment section.](#abandonment)
  
## Introduction
A credential broker service stores all sensitive information and has a command-line client which can act as a streaming pre-hook to initialize environment variables upon an application at runtime that does not store anything to disk. The broker service itself stores everything in encrypted format with the broker client having a key to unlock the data. The server requires a username, private PGP key & application name for it’s initial request. Upon the initial request if the user exists, matches the public key given, has access to the scope requested and has validated a time-based two-factor authentication within the last 24 hours it will return the encrypted data. After retrieval the client decrypts the data and sets the environment variables. If the two-factor authentication hasn’t occurred within the last 24 hours the server challenges the user first. All communications are over SSL.

Users can also be applications. A broker scope itself can be tied to an application such that an app server would use their private PGP key to match with the broker services stored public key to access all the variables stored under that applications scope while starting. Applications don't require two-factor authentication as they're trusted.

### Simple Example
![Success & Failure Example](/artwork/example1.svg)

### Authentication Sequence Diagram
![Authentication Sequence Diagram](/artwork/example2.svg)

## Server Setup
In order to start the server merely type `broker start`. It will ask a series of questions before it's operational. For more information on what these strategies mean and their reasons for existing, please see [the abandonment section.](#abandonment) If the user types `N/no` it validates their choice and gives them a warning specific to an abandonment situation that would leave their data without anyone who has access.

```
Allow strategy "oldest user acquires edit access"?
Allow strategy "revoker acquires abandoned scope access"?
Allow strategy "all users allowed to revoke"?
Allow strategy "first account gets all access"?
Allow strategy "any user can cancel a wipe"?
```

## Abandonment
There are four different scenarios or edge-cases that must be handled when it comes to transitioning access of sensitive data.

1. Person leaves company and their user is removed, being the only edit user of some scopes.
  - In this scenario the broker first attempts to give edit access to the oldest created user that has view access to the scope. In this scenario it emails all members who have any access to that scope and the person who revoked access of the original user what happened. This behavior can be disabled. This strategy is called `oldest user acquires edit access`.
  - If there are no users with view access to scopes created by deleted user, it gives edit access of the scopes to the operator who revoked access of the user and notifies all members that also have user edit access. This behavior can be disabled. This strategy is called `revoker acquires abandoned scope access`.
2. Person leaves company and their user can't be removed as there isn't another user with user edit access.
  - Any user on the broker may petition a revoking of another user. This sends an email to all users and gives the potentially revoked user 24hours to cancel the revoking before it removes the user. This behavior can be disabled. This strategy is called `all users allowed to revoke`. 
3. There is only one user in broker and they leave a company having deleted their own account first
  - The next user to create an account will acquire all scopes as edit access, user edit access and ability to create new scopes. This behavior can be disabled. This strategy is called `first account gets all access`.
4. There is only one user in broker and they leave a company without deleting their own account first, or there are many users on the broker but restoration strategies for maintaining a user with edit access has been disabled.
  - Somebody can run the `broker wipe` command to remove all users from the instance. All users with access to the broker will receive an email giving them 24hours to cancel the operation. This will allow somebody to create the first user and acquire access to all scopes unless `first account gets all access` strategy has been disabled. The ability to allow other users 24hours to cancel a wipe can be disabled and it's strategy is called `any user can cancel a wipe`.
5. Either situation 1 through 4 happened and their restoration behaviors were disabled, any user with access to the machine can output the scopes and sensitive item names but values will forever be encrypted so maybe it's time to rotate those keys or generate new ones. Acquiring the names still allows you to keep a map of the data lost, but by disabling the strategies above you run the risk of not being able to access the data should these situations occur.

## Types of Access
1. View access of other users
2. Edit access of other users
3. View access of all scope names
4. Create access of new scopes
5. ~View access of all scopes~ By Design: This is a violation of need-to-know
6. ~Edit access of all scopes~ By Design: This is a violation of need-to-know
7. View access of a specific scope
8. Edit access of a specific scope

## Commands
Below are a list of all the commands and descriptions of how they operate under various conditions.

```
# Start the broker server
# The client and server are the same code, what differentiates them is in order for
#  the server to work, it needs to be started.
# The broker server will first ask a series of question for restoration strategies
#  before it will start. You will create the first user with the client instance and
#  the command `broker init`
broker start

# Anyone with access to the server containing the broker service may trigger a 
#  wipe event. By default it gives 24hour warning to all users that a wipe will occur.
#  This warning may be disabled. See the ABANDONMENT section to learn more.
broker wipe

# Initialize the local account before using broker.
# It starts by asking for the remote address of the broker server
# It then asks for operator username
# It then asks for the location of operators private PGP key so it can automate logging in.
# It attempts to authenticate with the server to see if operator's username exists
# If the server connects and the account exists as a user it completes successful
# If the server connects and there are no existing accounts,
#  it gives the operator edit access to all existing scopes and scope create access & user
#  edit access unless this restoration strategy has been disabled. keep in mind that
#  just because they inherit all existing data, they won't be able to view or
#  edit any secrets or scopes that are created by others after this point unless those scopes
#  become abandoned. The purpose of this structure is to allow admins to inherit data that was 
#  abandoned by previous users, such as when employees leave a company. To read more about
#  these sensitive cases, refer to the ABANDONMENT section in the readme
# If the server connects and there are existing accounts, but the
#  operators account doesn't exist it informs the operator to contact the broker
#  admins and then lists the users that have user edit access
broker init

# Create a new user if the operator has user edit access
# If the user already exists it asks if operator wants to overwrite existing user.
# If the user doesn't exist it asks for the users public PGP key
# It then asks if user should have access to view or edit other users
# It then asks if user should have access to viewing all scope names
# It then asks if user should have access to create scopes
# It provides a list of scopes existing to allow user to access
# For every space the operator grants access, it asks whether access is view or edit
broker user add $userName

# Delete an existing user if the operator has user edit access
# If the user exists it validates that the operator indeed wants to delete the user.
broker user del $userName

# Modify an existing user if the operator has user edit access
# If the user exists it validates that the operator indeed wants to modify the user.
# It then asks if operator wants to change userName and if yes asks for an updated userName
# It then asks if operator wants to change public key and if yes asks for an updated public PGP key
# It then asks if user should have access to view or edit other users
# It then asks if user should have access to viewing all scope names
# It then asks if user should have access to create scopes
# It provides a list of scopes existing to allow user to access
# For every space the operator grants access, it asks whether access is view or edit
broker user mod $userName

# Get a list of all usernames if operator has access to viewing all users
broker user get

# Get a specific user and their information if operator has access to viewing all users
broker user get $userName

# Create a scope if user has scope admin and scope doesn't exist
# It validates that the user indeed intends to create an entire scope first
# It then asks if the scope itself can be a user, and then asks for a public PGP key if it can
broker scope add $scopeName

# Delete a scope if user has scope admin and scope exists
# It validates that the user indeed intends to delete an entire scope first
broker scope del $scopeName

# Rename a scope if user has scope admin and scope exists
# It validates that the user indeed intends to rename an entire scope first
# It then asks if the scope itself can be a user, and then asks for a public PGP key if it can
broker scope mod $scopeName

# Get a list of all scope names if operator has access to viewing all scope names
broker scope get
broker get

# Add a secret to a specified scope if the scope exists and user has scope edit access
# Asks if secret should have a TTL attached to it, if yes it asks for TTL value
# Asks for secret value, hiding the input
broker add $scopeName $secretName

# Deletes a secret from a specified scope if the scope exists and user has scope edit access
# It validates that the user indeed intends to delete the specified secret first
broker del $scopeName $secretName

# Changes a secret on a specified scope if the scope exists and user has scope edit access
# It first validates that the user indeed intends to modify an existing secret
# Asks if secret should have a TTL attached to it, if yes it asks for TTL value
# It then asks for secret value, hiding the input & then changes the secret on the specified scope
broker mod $scopeName $secretName

# Establish all environment variables based on scope if user has read access to specified scope
broker get $scopeName
```
