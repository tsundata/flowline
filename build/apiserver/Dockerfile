FROM alpine:latest

RUN apk update && apk add bash

COPY ./dist/apiserver_linux_amd64_v1/apiserver flowline-apiserver
CMD ["./flowline-apiserver"]