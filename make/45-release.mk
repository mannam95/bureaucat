##@ Release (publish the image to Docker Hub)

.PHONY: image-info image-login image-build image-push release release-check

image-info: ## Show exactly what would be built and pushed (dry run)
	@echo "image    : $(IMAGE)"
	@echo "tags     : $(VERSION) | sha-$(GIT_SHA) | latest"
	@echo "platform : $(PLATFORM)"
	@echo "baked in : main.Version=$(VERSION)"

image-login: ## Log in to Docker Hub (once per machine)
	docker login -u $(DOCKER_USER)

# The sha tag is only meaningful if the tree matches the commit it names.
release-check:
	@if [ -n "$$(git status --porcelain)" ] && [ -z "$(ALLOW_DIRTY)" ]; then \
	  echo "✗ working tree is dirty, so the sha-$(GIT_SHA) tag would not match what you built."; \
	  echo "  commit first, or override with: make release ALLOW_DIRTY=1"; \
	  exit 1; \
	fi

image-build: ## Build the release image, tagged version + sha + latest
	docker build \
	  --platform $(PLATFORM) \
	  --build-arg VERSION=$(VERSION) \
	  -t $(IMAGE):$(VERSION) \
	  -t $(IMAGE):sha-$(GIT_SHA) \
	  -t $(IMAGE):latest \
	  .

image-push: ## Push all three tags to Docker Hub
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):sha-$(GIT_SHA)
	docker push $(IMAGE):latest

release: release-check image-build image-push ## Build and push the release image (all tags)
	@echo "✓ published $(IMAGE)  [$(VERSION), sha-$(GIT_SHA), latest]"
