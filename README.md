# sriov-plugin

This repository contains a simple gRPC service for discovering SR-IOV capable network devices.
It parses the output of `lshw -class network -json` and enriches the data using the
[`gutil-linux`](https://github.com/TimRots/gutil-linux) library to read PCI information.

## Building

```
go build ./cmd/server
```

## Running

Start the gRPC server:

```
./server
```

In another terminal, run the example client:

```
go run ./cmd/client
```

The client will print a list of detected devices from the sample `lshw-network.json` file.
