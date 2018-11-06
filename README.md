# radix-api

The Radix API Server for accessing functionality on the Radix platform. For now the project has to be built locally. In order to build run 'make docker-build' command. If swagger annotation has been updated, run 'make swagger' to ensure it is following the code (you may need to delete ./swaggger_src/swagger.json and ./swagger/statik.go before you do that) 

## Common errors running locally

problem: panic: statik/fs: no zip data registered

solution: make swagger

problem: the version running in cluster is not the one you  expected

solution: you may have forgotten to build and pushed the latest changes

## Manual redeployment on existing cluster

### Prerequisites
1. Install draft
2. draft init from project directory (inside radix-api)
3. draft config set registry radixdev.azurecr.io
4. az acr login --name radixdev

### Process 
1. Make sure that the `kubernetes.go` in utils is not changed before you build
2. Update version in header of swagger version in `main.go` so that you can see that the version in the environment corresponds with what you wanted
3. Execute `make docker-build`
4. Execute `draft up` to install to dev environment of radix-api
5. Wait for pods to start
6. Go to https://server-radix-api-dev.<cluster name>.dev.radix.equinor.com/swaggerui/ to see if the version in the swagger corresponds with the version you set in the header