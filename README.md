# flowline

![Build](https://github.com/tsundata/flowline/workflows/Build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/tsundata/flowline)](https://goreportcard.com/report/github.com/tsundata/flowline)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/tsundata/flowline)
![GitHub](https://img.shields.io/github/license/tsundata/flowline)

Flowline is a workflow system for schedule

## Features

- Workflow base DAG
- RABC
- logs
- custom runtime

## Architecture

<img src="./docs/architecture.png" alt="Architecture" width="100%" /> 

## Applications used

- etcd

## Requirements

This project requires Go 1.18 or newer

## Installation

1. Install etcd

2. Import Configuration to etcd

3. Set Environment

4. Build binary

5. Run App binary

## Flowline REST API

[Swagger UI](https://tsundata.github.io/flowline/)

## [Flowline admin](https://github.com/tsundata/flowline-admin)

### dashboard
<img src="./docs/dashboard.png" width="100%"  alt=""/>

### workflows
<img src="./docs/workflows.png" width="100%"  alt=""/>

### dag editor
<img src="./docs/dag.png" width="100%"  alt=""/>

### users
<img src="./docs/users.png" width="100%"  alt=""/>

### runs
<img src="./docs/runs.png" width="100%"  alt=""/>

### jobs
<img src="./docs/jobs.png" width="100%"  alt=""/>

### events
<img src="./docs/events.png" width="100%"  alt=""/>

### workers
<img src="./docs/workers.png" width="100%"  alt=""/>

### codes
<img src="./docs/codes.png" width="100%"  alt=""/>

### variables
<img src="./docs/variables.png" width="100%"  alt=""/>

### connections
<img src="./docs/connections.png" width="100%"  alt=""/>

## Reference

[kubernetes](https://github.com/kubernetes/kubernetes)

## License

flowline is licensed under the [MIT license](https://opensource.org/licenses/MIT).
