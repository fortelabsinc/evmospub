#!/bin/sh


IMAGE="gcr.io/devops-project-allqd0/evmos:fi8be-0.0.1-0"
docker build -t ${IMAGE} .
sleep 2

docker push ${IMAGE}

