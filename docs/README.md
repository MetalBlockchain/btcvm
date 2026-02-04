## Overview

This repository uses a Makefile to automate building and running a local Metal network environment with the `metal-network-runner`. It also handles log output and cleanup tasks.

## Targets

- **build**  
  Builds the BTCVM plugin.  
  ```bash
  make build
  ```

- **run_local**  
  Runs the local network with five nodes, captures logs per node, and performs text substitutions on console output.  
  ```bash
  make run_local
  ```

- **cleanup**  
  Cleans up running processes and removes artifacts.  
  ```bash
  make cleanup
  ```

- **compile**  
  Watches for changes and rebuilds the Go code in `vm/main/`.  
  ```bash
  make compile
  ```

- **teardown**  
  Force-kills the network runner process.  
  ```bash
  make teardown
  ```

Use `make <target>` to run the commands above.


## How it works
- For detailed  instructions use `https://build.avax.network/docs/virtual-machines/golang-vms/complex-golang-vm`
- We create the vm which is under `vm/vm/vm.go`, it initializes the vm and runs the network

- This code defines a Bitcoin-based virtual machine (BTC VM) running on the Metal blockchain platform.
- It extends Metal’s Snowman consensus by embedding a local btcd instance to maintain Bitcoin’s state.
- The VM struct manages node context, database, mempool, block building, gossiping, and network interactions.
- Core methods include Initialize, Shutdown, SetState, LastAccepted, and block parsing/verification routines.
- This allows for a self-contained environment where Bitcoin transactions and blocks are synchronized using Metal’s consensus and network layers.
- It is configured to mint a block every 5 seconds by triggering it with a timer
