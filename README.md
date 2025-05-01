# External Chaincode Builder and Launcher for Kubernetes

This is chaincode builder that uses **Kubernetes bare pods** to build and run [Hyperledger Fabric](https://www.hyperledger.org/use/fabric) chaincode.

The goal is to provide a Kubernetes-native way to handle external builders and launchers for Fabric chaincode deployments.

---

## Features

- Uses Kubernetes bare pods for chaincode build and execution.
- Integrates with the [Fabric Operator](https://github.com/hyperledger-labs/fabric-operator) and the [Fabric Operations Console](https://github.com/hyperledger-labs/fabric-operations-console).
- Simplifies development workflows on Kubernetes without relying on Docker-in-Docker or privileged containers.

---

## Getting Started



### Run the Tests

Once Kubernetes is enabled on Docker Desktop:

```bash
go test ./...
```

---

## Usage with Fabric Operator & Fabric Operations Console

This external builder is compatible with both:

- [Fabric Operator](https://github.com/hyperledger-labs/fabric-operator): Automates lifecycle management of Fabric networks on Kubernetes.
- [Fabric Operations Console](https://github.com/hyperledger-labs/fabric-operations-console): A GUI-based tool to interact with and manage Hyperledger Fabric networks.

You can configure this external chaincode builder within your operator or console deployment for custom chaincode packaging and deployment flows.

---

## License

[Apache 2.0](LICENSE)
