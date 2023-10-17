![prod](https://api.radix.equinor.com/api/v1/applications/radix-api//environments/prod/buildstatus)
![qa](https://api.radix.equinor.com/api/v1/applications/radix-api//environments/qa/buildstatus)
# Radix API

The Radix API is an HTTP server for accessing functionality on the [Radix](https://www.radix.equinor.com) platform. This document is for Radix developers, or anyone interested in poking around. 

## Purpose

The Radix API is meant to be the single point of entry for platform users to the platform (through e.g. the Web Console). Users should not be able to access the Kubernetes API directly; therefore the Radix API limits and customises what platform users are able to do.

## Security

Authentication and authorisation are performed through an HTTP bearer token, which is relayed to the Kubernetes API. The Kubernetes AAD integration then performs its authentication and resource authorisation checks, and the result is relayed to the user.

## Developing

You need Go installed. Make sure `GOPATH` and `GOROOT` are properly set up.

Also needed:

- [`go-swagger`](https://github.com/go-swagger/go-swagger) (install with `go install github.com/go-swagger/go-swagger/cmd/swagger@v0.30.5`.)
- [`statik`](https://github.com/rakyll/statik) (install with `go install github.com/rakyll/statik@v0.1.7`)
- [`gomock`](https://github.com/golang/mock) (install with `go install github.com/golang/mock/mockgen@v1.6.0`)

Clone the repo into your `GOPATH` and run `go mod download`.

### Dependencies - go modules

Go modules are used for dependency management. See [link](https://blog.golang.org/using-go-modules) for information how to add, upgrade and remove dependencies. E.g. To update `radix-operator` dependency:

- list versions: `go list -m -versions github.com/equinor/radix-operator`
- update: `go get github.com/equinor/radix-operator@v1.3.1`

### Generating mocks
We use gomock to generate mocks used in unit test.
You need to regenerate mocks if you make changes to any of the interface types used by the application.
```
make mocks
```

### Running locally

The following env vars are needed. Useful default values in brackets.

- `RADIX_CONTAINER_REGISTRY` - (`radixdev.azurecr.io`)
- `PIPELINE_IMG_TAG` - (`master-latest`)

You also probably want to start with the argument `--useOutClusterClient=false`. When `useOutClusterClient` is `false`, several debugging settings are enabled:
* a service principal with superpowers is used to authorize the requests, and the client's `Authorization` bearer token is ignored. 
* the Radix API will connect to the currently-configured `kubectl` context and ignore `K8S_API_HOST`.
* the server CORS settings are modified to accept the `X-Requested-With` header in incoming requests. This is necessary to allow direct requests from web browser while e.g. debugging [radix-web-console](https://github.com/equinor/radix-web-console).
* verbose debugging output from CORS rule evaluation is logged to console.

If you are using VSCode, there is a convenient launch configuration in `.vscode`.

#### Common errors running locally

- **Problem**: `panic: statik/fs: no zip data registered`

  **Solution**: `make swagger`

#### Validate code

- `go install honnef.co/go/tools/cmd/staticcheck@v0.3.3`
- run `make staticcheck`

#### Update version
We follow the [semantic version](https://semver.org/) as recommended by [go](https://blog.golang.org/publishing-go-modules).
`radix-api` has three places to set version
* `apiVersionRoute` in `api/router/server.go` and `BasePath`in `docs/docs.go` - API version, used in API's URL
* `Version` in `docs/docs.go` - indicates changes in radix-api logic - to see (e.g. in swagger), that the version in the environment corresponds with what you wanted

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

## Contributing

Read our [contributing guidelines](./CONTRIBUTING.md)

------------------

[Security notification](./SECURITY.md)