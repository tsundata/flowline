FROM alpine:latest

RUN apk update && apk add bash

COPY ./dist/controller-manager_linux_amd64_v1/controller-manager flowline-controller-manager
CMD ["./flowline-controller-manager"]