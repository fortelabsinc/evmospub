#!/bin/sh


IMAGE_NAME="fi8be"
PROJECT_NAME="devops-project-allqd0"
TAG_VERSION="v0.0.0-1"


docker build -t $IMAGE_NAME .
sleep 2
# Tag the build
docker tag $IMAGE_NAME gcr.io/$PROJECT_NAME/$IMAGE_NAME:$TAG_VERSION
sleep 2

# Puth the image to the gcr
docker push gcr.io/$PROJECT_NAME/$IMAGE_NAME:$TAG_VERSION

