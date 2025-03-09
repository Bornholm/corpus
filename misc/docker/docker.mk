DOCKER_IMAGE_NAME := corpus
DOCKER_IMAGE_TAG := latest

docker-image:
	docker build -f misc/docker/Dockerfile -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) .

docker-run:
	docker run -it --rm  --name corpus --env-file .env $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)