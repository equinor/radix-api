# Radix API

The Radix API is an HTTP server for accessing functionality on the [Radix](https://www.radix.equinor.com) platform. This document is for Radix developers, or anyone interested in poking around. Please see [Development practices](./development-practices.md) for more information on the release process.

## Purpose

The Radix API is meant to be the single point of entry for platform users to the platform (through e.g. the Web Console). Users should not be able to access the Kubernetes API directly; therefore the Radix API limits and customises what platform users are able to do.

## Security

Authentication and authorisation are performed through an HTTP bearer token, which is (in most cases) relayed to the Kubernetes API. The Kubernetes AAD integration then performs its authentication and resource authorisation checks, and the result is relayed to the the user.

Some requests trigger more complex authorisation checks within the Radix API itself by using the `radix-api` [clusterrole](https://github.com/equinor/radix-operator/blob/master/docs/RBAC.md#api).

## Developing

You need Go installed. Make sure `GOPATH` and `GOROOT` are properly set up.

Also needed:

- [`go-swagger`](https://github.com/go-swagger/go-swagger) (on a Mac, you can install it with Homebrew: `brew install go-swagger`)
- [`statik`](https://github.com/rakyll/statik) (install with `go get github.com/rakyll/statik`)

Clone the repo into your `GOPATH` and run `go mod download`.

### Dependencies - go modules

Go modules are used for dependency management. See [link](https://blog.golang.org/using-go-modules) for information how to add, upgrade and remove dependencies. E.g. To update `radix-operator` dependency:

- list versions: `go list -m -versions github.com/equinor/radix-operator`
- update: `go get github.com/equinor/radix-operator@v1.3.1`

### Generating mocks
We use gomock to generate mocks used in unit test.
You need to regenerate mocks if you make changes to any of the interface types used by the application; **Status**

Status:
```
$ mockgen -source ./api/buildstatus/models/buildstatus.go -destination ./api/test/mock/buildstatus_mock.go -package mock
```

### Running locally

The following env vars are needed. Useful default values in brackets.

- `RADIX_CONTAINER_REGISTRY` - (`radixdev.azurecr.io`)
- `PIPELINE_IMG_TAG` - (`master-latest`)

You also probably want to start with the argument `--useOutClusterClient=false`. If this is set to `true` (the default) the program will connect to the K8S API host defined by the `K8S_API_HOST` env var and will require auth tokens in all client requests. Set to `false`, a service principal with superpowers is used to authorise the requests instead (**you still need to send** a `bearer whatever` auth header with the requests, but its value is ignored).

When `useOutClusterClient` is `false`, the Radix API will connect to the currently-configured `kubectl` context.

If you are using VSCode, there is a convenient launch configuration in `.vscode`.

#### Common errors running locally

- **Problem**: `panic: statik/fs: no zip data registered`

  **Solution**: `make swagger`

#### Update version
We follow the [semantic version](https://semver.org/) as recommended by [go](https://blog.golang.org/publishing-go-modules).
`radix-api` has three places to set version
* `apiVersionRoute` in `api/router/server.go` and `BasePath`in `docs/docs.go` - API version, used in API's URL
* `Version` in `docs/docs.go` - indicates changes in radix-api logic - to see (e.g in swagger), that the version in the environment corresponds with what you wanted

    Run following command to update version in `swagger.json`
    ```
    make swagger
    ``` 

* `tag` in git repository (in `master` branch) - matching to the version of `Version` in `docs/docs.go`

    Run following command to set `tag` (with corresponding version)
    ```
    git tag v1.0.0
    git push origin v1.0.0
    ```

### Manual redeployment on existing cluster

#### Prerequisites

1. Install draft (https://draft.sh/)
2. `draft init` from project directory (inside `radix-api`)
3. `draft config set registry radixdev.azurecr.io`
4. `az acr login --name radixdev`

#### Process

1. Update version
2. Execute `draft up` to install to dev environment of radix-api
3. Wait for pods to start
4. Go to `https://server-radix-api-dev.<cluster name>.dev.radix.equinor.com/swaggerui/` to see if the version in the swagger corresponds with the version you set in the header.

## Security Principle

The Radix API server is meant to be the single point of entry for platform users to the platform (through the web console or a command line interface). They should not be able to access the Kubernetes API directly. Therefore the Radix API will limit what platform users will be able to do. Authentication is done through a bearer token, which essentially is relayed to the Kubernetes API to ensure that users only can see what they should be able to see, and therefore rely on the k8s AAD integration for authentication <sup><sup>1</sup></sup>.

<sup><sup>1</sup></sup> <sub><sup>Until the work referred to in [this document](https://github.com/equinor/radix-operator/blob/master/docs/RBAC.md) is solved, listing applications, listing jobs and creating build job is done using the service account of the API server, and access is therefore verified inside the Radix API server rather than by the Kubernetes API using RBAC.</sup></sub>

## Deployment

Radix API follows the [standard procedure](https://github.com/equinor/radix-private/blob/master/docs/how-we-work/development-practices.md#standard-radix-applications) defined in _how we work_.

Radix API is installed as a Radix application in [script](https://github.com/equinor/radix-platform/blob/master/scripts/install_base_components.sh) when setting up a cluster. It will setup API environment with [aliases](https://github.com/equinor/radix-platform/blob/master/scripts/create_alias.sh), and a Webhook so that changes to this repository will be reflected in Radix platform.
```
If radix-operator is updated to a new tag, `go.mod` should be updated as follows: 
   
    github.com/equinor/radix-operator <NEW_OPERATOR_TAG>
```
## Pull request checking

Radix API makes use of [GitHub Actions](https://github.com/features/actions) for build checking in every pull request to the `master` branch. Refer to the [configuration file](https://github.com/equinor/radix-api/blob/master/.github/workflows/radix-api-pr.yml) of the workflow for more details.
