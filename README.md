# radix-api

The Radix API Server for accessing functionality on the Radix platform. Please see [Development practices](./development-practices.md) for more information on the release process.

## Developing

You need Go and [`dep`](https://github.com/golang/dep) installed. Make sure `GOPATH` and `GOROOT` are properly set up.

Also needed:

  - [`go-swagger`](https://github.com/go-swagger/go-swagger) (on a Mac, you can install it with Homebrew: `brew install go-swagger`)
  - [`statik`](https://github.com/rakyll/statik) (install with `go get github.com/rakyll/statik`)

Clone the repo into your `GOPATH` and run `dep ensure`.

## Common errors running locally

**Problem**: `panic: statik/fs: no zip data registered`

**Solution**: `make swagger`

## Deployment

Radix API follows the [standard procedure](https://github.com/equinor/radix-pƒrivate/blob/master/docs/how-we-work/development-practices.md#standard-radix-applications) defined in _how we work_.

Radix API is installed as a Radix application in [script](https://github.com/equinor/radix-platform/blob/master/scripts/install_base_components.sh) when setting up a cluster. It will setup API environment with [aliases](https://github.com/equinor/radix-platform/blob/master/scripts/create_alias.sh), and a Webhook so that changes to this repository will be reflected in Radix platform.

## Running locally

The following env vars are needed. Useful default values in brackets.

- `server_cert_path` - (`${workspaceFolder}/certs/cert.pem`)
- `server_key_path` - (`${workspaceFolder}/certs/key.pem`)
- `RADIX_CONTAINER_REGISTRY` - (`radixdev.azurecr.io`)
- `PIPELINE_IMG_TAG` - (`master-latest`)

You also probably want to start with the argument `--useOutClusterClient=false`. If this is set to `true` (the default) the program will connect to the K8S API host defined by the `K8S_API_HOST` env var and will require auth tokens in all client requests. Set to `false`, a service principal with superpowers is used to authorise the requests instead (**you still need to send** a `bearer whatever` auth header with the requests, but its value is ignored).

When `useOutClusterClient` is `false`, the Radix API will connect to the currently-configured `kubectl` context.

If you are using VSCode, there is a convenient launch configuration in `.vscode`.

## Manual redeployment on existing cluster

### Prerequisites

1. Install draft (https://draft.sh/)
2. `draft init` from project directory (inside `radix-api`)
3. `draft config set registry radixdev.azurecr.io`
4. `az acr login --name radixdev`

### Process

1. Update version in header of swagger version in `main.go` so that you can see that the version in the environment corresponds with what you wanted
2. Execute `draft up` to install to dev environment of radix-api
3. Wait for pods to start
4. Go to `https://server-radix-api-dev.<cluster name>.dev.radix.equinor.com/swaggerui/` to see if the version in the swagger corresponds with the version you set in the header.

## Security Principle

The Radix API server is meant to be the single point of entry for platform users to the platform (through the web console or a command line interface). They should not be able to access the Kubernetes API directly. Therefore the Radix API will limit what platform users will be able to do. Authentication is done through a bearer token, which essentially is relayed to the Kubernetes API to ensure that users only can see what they should be able to see, and therefore rely on the k8s AAD integration for authentication <sup><sup>1</sup></sup>.

<sup><sup>1</sup></sup> <sub><sup>Until the work referred to in [this document](https://github.com/equinor/radix-operator/blob/master/docs/RBAC.md) is solved, listing applications, listing jobs and creating build job is done using the service account of the API server, and access is therefore verified inside the Radix API server rather than by the Kubernetes API using RBAC.</sup></sub>
