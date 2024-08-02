<img src="https://assets-global.website-files.com/5f734f4dbd95382f4fdfa0ea/65b0115890bbda5c804f7524_donuts%202-p-500.png" alt="evm" width="300"/>

# EVM Gateway

**EVM Gateway enables seamless interaction with EVM on Flow, mirroring the experience of engaging with any other EVM blockchain.**

EVM Gateway implements the Ethereum JSON-RPC API for [EVM on Flow](https://developers.flow.com/evm/about) which conforms to the Ethereum [JSON-RPC specification](https://ethereum.github.io/execution-apis/api-documentation/). EVM Gateway is specifically designed to integrate with the EVM environment on the Flow blockchain. Rather than implementing the full `geth` stack, the JSON-RPC API available in EVM Gateway is a lightweight implementation which uses Flow's underlying consensus and smart contract language, [Cadence](https://cadence-lang.org/docs/), to handle calls received by the EVM Gateway. For those interested in the underlying implementation details please refer to the [FLIP #243](https://github.com/onflow/flips/issues/243) (EVM Gateway) and [FLIP #223](https://github.com/onflow/flips/issues/223) (EVM on Flow Core) improvement proposals. 

EVM Gateway is compatible with the majority of standard Ethereum JSON-RPC APIs allowing seamless integration with existing Ethereum-compatible web3 tools via HTTP. EVM Gateway honors Ethereum's JSON-RPC namespace system, grouping RPC methods into categories based on their specific purpose. Each method name is constructed using the namespace, an underscore, and the specific method name in that namespace. For example, the `eth_call` method is located within the `eth` namespace. See below for details on methods currently supported or planned.

### Design

![design ](https://github.com/onflow/flow-evm-gateway/assets/75445744/3fd65313-4041-46d1-b263-b848640d019f)


The basic design of the EVM Gateway consists of a couple of components:

- Event Ingestion Engine: this component listens to all Cadence events that are emitted by the EVM core, which can be identified by the special event type ID `evm.TransactionExecuted` and `evm.BlockExecuted` and decodes and index the data they contain in the payloads.
- Flow Requester: this component knows how to submit transactions to Flow AN to change the EVM state. What happens behind the scenes is that EVM gateway will receive an EVM transaction payload, which will get wrapped in a Cadence transaction that calls EVM contract with that payload and then the EVM core will execute the transaction and change the state.
- JSON RPC API: this is the client API component that implements all the API according to the JSON RPC API specification.

## Event subscription and filters

EVM Gateway also supports the standard Ethereum JSON-RPC event subscription and filters, enabling callers to subscribe to state logs, blocks or pending transactions changes.

* TODO more coming

# Running
Operating an EVM Gateway is straightforward. It can either be deployed locally alongside the Flow emulator or configured to connect with any active Flow networks supporting EVM. Given that the EVM Gateway depends solely on [Access Node APIs](https://developers.flow.com/networks/node-ops/access-onchain-data/access-nodes/accessing-data/access-api), it is compatible with any networks offering this API access.

### Running Locally
**Start Emulator**

In order to run the gateway locally you need to start the emulator with EVM enabled:
```
flow emulator --evm-enabled
```
_Make sure flow.json has the emulator account configured to address and private key we will use for starting gateway bellow._

Then you need to start the gateway:
```
go run cmd/main/main.go \
  --flow-network-id flow-emulator \
  --coinbase FACF71692421039876a5BB4F10EF7A439D8ef61E \
  --coa-address f8d6e0586b0a20c7 \
  --coa-key 2619878f0e2ff438d17835c2a4561cb87b4d24d72d12ec34569acd0dd4af7c21 \
  --coa-resource-create \
  --gas-price 0
```

Note that the gateway will be starting from the latest emulator block, so if emulator is run before and transactions happen in the meantime, the gateway will not fetch those historical blocks & transactions.
This will be improved soon.

_In this example we use `coa-address` value set to service account of the emulator, same as `coa-key`. 
This account will by default be funded with Flow which is a requirement. For `coinbase` we can 
use whichever valid EVM address. It's not really useful for local running beside collecting fees. We provide also the 
`coa-resource-create` to auto-create resources needed on start-up on the `coa` account in order to operate gateway. 
`gas-price` is set at 0 so we don't have to fund EOA accounts. We can set it higher but keep in mind you will then 
need funded accounts for interacting with EVM._

**With Docker**

Run the following commands:

```bash
cd dev

docker build -t onflow/flow-evm-gateway .

docker run -d -p 127.0.0.1:8545:8545 onflow/flow-evm-gateway
```

To verify the service is up and running:

```bash
curl -XPOST 'localhost:8545'  --header 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

it should return:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x2"
}
```

## Configuration Flags

The application can be configured using the following flags at runtime:

| Flag                        | Default Value    | Description                                                                                                                                     |
|-----------------------------|------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `--database-dir`            | `./db`           | Path to the directory for the database.                                                                                                         |
| `--rpc-host`                | `localhost`      | Host for the JSON RPC API server.                                                                                                               |
| `--rpc-port`                | `8545`           | Port for the JSON RPC API server.                                                                                                               |
| `--ws-enabled`              | `false`          | Enable websocket support.                                                                                                                       |
| `--access-node-grpc-host`   | `localhost:3569` | Host to the current spork Flow access node (AN) gRPC API.                                                                                       |
| `--access-node-spork-hosts` |                  | Previous spork AN hosts, defined following the schema: `{latest height}@{host}` as comma separated list (e.g. `"200@host-1.com,300@host2.com"`) |
| `--evm-network-id`          | `testnet`        | EVM network ID (options: `testnet`, `mainnet`).                                                                                                 |
| `--flow-network-id`         | `emulator`       | Flow network ID (options: `emulator`, `previewnet`).                                                                                            |
| `--coinbase`                | (required)       | Coinbase address to use for fee collection.                                                                                                     |
| `--init-cadence-height`     | 0                | Define the Cadence block height at which to start the indexing.                                                                                 |
| `--gas-price`               | `1`              | Static gas price used for EVM transactions.                                                                                                     |
| `--coa-address`             | (required)       | Flow address that holds COA account used for submitting transactions.                                                                           |
| `--coa-key`                 | (required)       | *WARNING*: Do not use this flag in production! Private key value for the COA address used for submitting transactions.                          |
| `--coa-key-file`            |                  | File path that contains JSON array of COA keys used in key-rotation mechanism, this is exclusive with `coa-key` flag.                           |
| `--coa-resource-create`     | `false`          | Auto-create the COA resource in the Flow COA account provided if one doesn't exist.                                                             |
| `--log-level`               | `debug`          | Define verbosity of the log output ('debug', 'info', 'error')                                                                                   |
| `--stream-limit`            | 10               | Rate-limits the events sent to the client within one second                                                                                     |
| `--stream-timeout`          | 3sec             | Defines the timeout in seconds the server waits for the event to be sent to the client                                                          |
| `--filter-expiry`           | `5m`             | Filter defines the time it takes for an idle filter to expire                                                                                   |

## Getting Started

To start using EVM Gateway, ensure you have the required dependencies installed and then run the application with your desired configuration flags. For example:

```bash
./evm-gateway --rpc-host "127.0.0.1" --rpc-port 3000 --database-dir "/path/to/database"
````
For more detailed information on configuration and deployment, refer to the Configuration and Deployment sections.

# EVM Gateway Endpoints

EVM Gateway has public RPC endpoints available for the following environments:

| Name            | Value                                  |
|-----------------|----------------------------------------|
| Network Name    | Migrationnet                             |
| Description     | The public RPC URL for Flow Migrationnet |
| RPC Endpoint    | https://evm-001.migrationtestnet1.nodes.onflow.org|
| Chain ID        | 646                                    |
| Currency Symbol | FLOW                                   |
| Block Explorer  | /        |

| Name            | Value                                  |
|-----------------|----------------------------------------|
| Network Name    | Previewnet                             |
| Description     | The public RPC URL for Flow Previewnet |
| RPC Endpoint    | https://previewnet.evm.nodes.onflow.org|
| Chain ID        | 646                                    |
| Currency Symbol | FLOW                                   |
| Block Explorer  | https://previewnet.flowdiver.io        |

| Name            | Value                                  |
|-----------------|----------------------------------------|
| Network Name    | Testnet                                |
| Description     | The public RPC URL for Flow Testnet    |
| RPC Endpoint    | https://testnet.evm.nodes.onflow.org   |
| Chain ID        | Coming Soon                            |
| Currency Symbol | FLOW                                   |
| Block Explorer  | https://testnet.flowdiver.io           |

| Name            | Value                                  |
|-----------------|----------------------------------------|
| Network Name    | Mainnet                                |
| Description     | The public RPC URL for Flow Mainnet    |
| RPC Endpoint    | https://mainnet.evm.nodes.onflow.org   |
| Chain ID        | 747                                    |
| Currency Symbol | FLOW                                   |
| Block Explorer  | https://flowdiver.io                   |


# JSON-RPC API
The EVM Gateway implements APIs acording to the Ethereum specification: https://ethereum.org/en/developers/docs/apis/json-rpc/#json-rpc-methods

**Additional APIs**
Beside the APIs from the specification we support some additional APIs:
- Tracing APIs allow you to fetch execution traces
    * debug_traceTransaction
    * debug_traceBlockByNumber
    * debug_traceBlockByHash
 
**Unsuported APIs**
- Wallet APIs: we don't officialy support wallet APIs (eth_accounts, eth_sign, eth_signTransaction, eth_sendTransaction) due to security
  concerns that come with managing the keys on production environments, however it is possible to configure the gateway to allow these
  methods for local development by using a special flag `--wallet-api-key`. 
- Proof API: we don't support obtaining proofs yet, Flow piggy-backs on the Flow consensus and hence the Flow proofs can be used to verify
  and trust the EVM environment. We intend to add access to EVM proofs in the future.
- Access Lists: we don't yet support creating access lists as they don't affect the fees we charge. We might support this in the future
  to optimize fees, but it currently is not part of our priorities. 


# Contributing
We welcome contributions from the community! Please read our [Contributing Guide](./CONTRIBUTING.md) for information on how to get involved.

# License
EVM Gateway is released under the Apache License 2.0 license. See the LICENSE file for more details.
