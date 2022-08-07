FROM alpine:latest

RUN apk update && apk add bash

COPY ./dist/scheduler_linux_amd64_v1/scheduler flowline-scheduler
CMD ["./flowline-scheduler"]