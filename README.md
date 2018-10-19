# radix-api

The Radix API Server for accessing functionality on the Radix platform. For now the project has to be built locally. In order to build run 'make docker-build' command. If swagger annotation has been updated, run 'make swagger' to ensure it is following the code (you may need to delete ./swaggger_src/swagger.json and ./swagger/statik.go before you do that) 

## Common errors running locally

problem: panic: statik/fs: no zip data registered

solution: make swagger

problem: the version running in cluster is not the one you  expected

solution: you may have forgotten to build and pushed the latest changes.
