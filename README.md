# radix-api

The Radix API Server for accessing functionality on the Radix platform. For now the project has to be built locally. In order to build run 'make docker-build' command. If swagger annotation has been updated, run 'make swagger' to ensure it is following the code (you may need to delete ./swaggger_src/swagger.json and ./swagger/statik.go before you do that) 

## Common errors running locally

problem: panic: statik/fs: no zip data registered

solution: make swagger

problem: the version running in cluster is not the one you  expected

solution: you may have forgotten to build and pushed the latest changes

## Manual redeployment on existing cluster

1. Make sure that the `kubernetes.go` in utils is not changed before you build
2. Update version in header of swagger version in `main.go` so that you can see that the version in the environment corresponds with what you wanted
3. Execute `make docker-build`
4. Execute `docker images` to see the imagetag of the last build
5. Execute `az acr login --name radixdev`
6. Execute `docker push radixdev.azurecr.io/radix-api:<imagetag>` to push the image created in step 3
7. Execute `kubectl edit deploy server -n radix-api-qa`
8. Edit the image name from `radix-api-server` to `radix-api` and tag from `latest` to `<imagetag>`
9. Save and close
10. Wait for pods to start
11. Go to https://server-radix-api-qa.<cluster name>.dev.radix.equinor.com/swaggerui/ to see if the version in the swagger corresponds with the version you set in the header