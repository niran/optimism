# Domain Modelling

To make tests easy to understand, as much as possible, the DSL uses well-understood concepts and common terminology
in API definitions. This page outlines the core concepts and terms that are fundamental to the DSL. Note that this is
not intended to be an exhaustive list - most concepts are implemented directly in the DSL code and don't need any
further explanation here.

## Key Concepts

### User

A _user_ is a network-agnostic end user. Typically this is a specific private key which may then be used to send
transactions on various L1 or L2 networks.

Importantly, this is the blockchain view of a user, not an attempt at representing a single human or off-chain entity.
Two different private keys are two different users. Accounts on different networks are considered to belong to the same
user if they are equivalent to the system. For example a deposit transaction from an L1 EOA credits funds to the same
address on the L2 so those two wallets belong to the same user.

### Wallet

A _wallet_ is the representation of a _user_ on a specific chain. Transactions are sent from wallets and included on the
chain relevant to that wallet.

### Contract

A _contract_ is the chain-agnostic representation of a contract. A _wallet_ can send a transaction that deploys the
contract to a specific chain which creates a _contract deployment_.

### Contract Deployment

A _contract deployment_ is the deployment of a _contract_ on a specific network. There may be multiple deployments of
the same contract on the same chain and/or across multiple chains.

### Network

A _network_ is an L1 or L2 blockchain. There can be many _nodes_ on each network.

### Node

A single node on a specific network. Consists of an execution and consensus client pair, potentially with a supervisor.

## "Entry-point" Order of Preference

When defining a new API there are often multiple options for which object the method is on and which it accepts as
parameters. For example a simple ETH transfer could be `l2EL.SendETH(fromWallet, toWallet)`,
`wallet.SendETH(l2EL, targetWallet)`, or just `wallet.SendETH(targetWallet)` with an optional argument being available
to specific a specific EL to send the transaction to. When such choices arise, it is important for the DSL to be
consistent so the following order of preference is used:

1. User
2. Wallet
3. Contract
4. Contract Deployment
5. Network
6. Execution Client
7. Consensus Client
8. Supervisor

For the example above `wallet.SendETH(targetWallet)` would be preferred. Since the wallet is network-specific a
node from that network can be automatically selected and used as a default. While `wallet.SendETH(targetUser)` could be
used, it is better to be consistent with the types used.

While `user.SendETH(targetUser)` might make sense when the interop is fully realised and everything is chain-agnostic,
for testing purposes it is likely too abstract as readers of the test can't tell which chain is being used.

Common sense should still be used to ensure that APIs added to specific types actually make sense. For example avoid
`wallet.WaitForBlock(x)`. While the wallet is specific to a network so could theoretically implement this, it makes far
more sense to have `network.WaitForBlock` and/or `executionClient.WaitForBlock`.

### Additional Examples

Applying the order of preference leads to:

* `wallet.Deploy(contract)` rather than `contract.Deploy(wallet)`
  * Contracts which are not owned should expose `contract.Deploy(network)` and use a generic account to deploy.
* `user.SendTx(deployment.PerformAction)` rather than `deployment.PerformAction(user)` (really????)
* `wallet.VerifyBalance(expectedAmount)` rather than `network.VerifyBalance(user, expectedAmount)`
  * `user.VerifyTotalBalance(expectedAmount)` to verify total ETH across all chains may also be useful
  * Avoid `user.VerifyTotalBalance(network, expectedAmount)`
