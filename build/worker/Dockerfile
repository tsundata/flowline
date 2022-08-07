FROM alpine:latest

RUN apk update && apk add bash

COPY ./dist/worker_linux_amd64_v1/worker flowline-worker
CMD ["./flowline-worker"]