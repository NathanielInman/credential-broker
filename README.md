# Credential Broker
A secure and easy way to manage adding, deleting and retrieving sensitive information

## Usage

```bash
# Asks for secret value, hiding the input & then establishes secret on the specified scope
broker add $scopeName $secretName

# Deletes a secret from specified scope
broker del $scopeName $secretName

# Asks for secret value, hiding the input & then changes a secret on the specified scope
broker mod $scopeName $secretName

# Establish all environment variables based on scope if authentication is successful
broker get $scopeName
```

## Benefits & Reasoning
  - Devops shouldn't implicitly have access to the information 
    - Just because an employee has access to the infrastructure doesn't mean they have a need-to-know.
  - Just because a user has access to one application doesn't mean they have access to all it's data
    - Data can be scoped to a need-to-know.
  - No knowledge of a language or configuration structure is required, just 4 command.
  - Adding, removing and changing secrets should be faster than unvaulting & editing playbooks
  - The responsibility of adding and removing secrets can be managed by organizational structure
    - Just because a developer has access to the secrets to run an application doesn't mean they have access to changing, adding or removing additional secrets.
  - The system can be used to store non-sensitive information
  
## Introduction
A credential broker service stores all sensitive information and has a client which can act as a streaming pre-hook to initialize environment variables upon an application at runtime that does not store anything to disk. The broker server itself stores everything in encrypted format with the broker client having a key to unlock the data. The server requires a username, private PGP key & application name for it’s initial request. Upon the initial request if the user exists, matches the public key given, has access to the scope requested and has validated a time-based two-factor authentication within the last 24 hours it will return the encrypted data. After retrieval the client decrypts the data and sets the environment variables. If the two-factor authentication hasn’t occurred within the last 24 hours the server challenges the user first. All communications are over SSL.

### Simple Example
![Success & Failure Example](/artwork/example1.svg)

### Authentication Sequence Diagram
![Authentication Sequence Diagram](/artwork/example2.svg)
