# radix-api

The Radix API Server for accessing functionality on the Radix platform. Please see [Development practices](./development-practices.md) for more information on the release process.

## Common errors running locally

problem: panic: statik/fs: no zip data registered

solution: make swagger

## Manual redeployment on existing cluster

### Prerequisites
1. Install draft (https://draft.sh/)
2. `draft init` from project directory (inside `radix-api`)
3. `draft config set registry radixdev.azurecr.io`
4. `az acr login --name radixdev`

### Process 
1. Update version in header of swagger version in `main.go` so that you can see that the version in the environment corresponds with what you wanted
3. Execute `draft up` to install to dev environment of radix-api
4. Wait for pods to start
5. Go to `https://server-radix-api-dev.<cluster name>.dev.radix.equinor.com/swaggerui/` to see if the version in the swagger corresponds with the version you set in the header
