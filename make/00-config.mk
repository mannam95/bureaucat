##
## Shared configuration
##

# Recipes need bash (for `until`, brace groups, etc.)
SHELL := bash

# Compose invocations. Projects are explicitly named so `make nuke` can target
# exactly this repo's resources and nothing else.
DC      := docker compose -p bureaucat
DC_PROD := docker compose -p bureaucat-prod -f docker-compose.prod.yml

# Key dev service names + the air-built binary inside the app container
APP := app
PG  := postgres
BIN := /app/tmp/main

# App URL + health endpoint used to wait for readiness
APP_URL    := http://localhost:1341
HEALTH_URL := $(APP_URL)/api/v1/health

# Local files created by `make setup`
ENV_FILE    := .env
ENV_EXAMPLE := .env.example
GARAGE_TOML := garage/garage.toml

# Host Python for tooling (portable; override with `make seed PYTHON=...`)
PYTHON := python3

# Published image on Docker Hub. Every value is overridable ad-hoc, e.g.
# `make release VERSION=1.2.0`. VERSION is also baked into the binary via
# ldflags, so `bureaucat --version` matches the image tag.
# Bump VERSION here as part of each release so a bare `make release` never
# re-publishes an already-released tag from newer code.
DOCKER_USER ?= mvsrinath
IMAGE_NAME  ?= sprintboard
VERSION     ?= 1.2.0
IMAGE        = docker.io/$(DOCKER_USER)/$(IMAGE_NAME)
GIT_SHA     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
PLATFORM    ?= linux/amd64
