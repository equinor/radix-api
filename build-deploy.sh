set -e
tag_name=anneli-$(uuidgen)
echo "tag name is $tag_name"
image_name=radixdev.azurecr.io/radix-api-server:$tag_name
docker build -f Dockerfile.dev . -t $image_name && docker push $image_name /
&& rd_name=$(kubectl get rd -n radix-api-qa -ocustom-columns=name:.metadata.name,active:.status.condition | grep Active | awk '{print $1}') /
&& kubectl patch rd $rd_name -n radix-api-qa --type='json' -p='[{"op": "replace", "path": "/spec/components/0/image", "value": "'$image_name'"}]'