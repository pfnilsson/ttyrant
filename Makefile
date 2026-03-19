.PHONY: install run patch minor major

install:
	go install .
	ttyrant install-hooks

run:
	go build -o ttyrant . && ./ttyrant

LATEST_TAG := $(shell git tag -l 'v*' --sort=-v:refname | head -1)
VERSION := $(if $(LATEST_TAG),$(LATEST_TAG),v0.0.0)
MAJOR := $(word 1,$(subst ., ,$(subst v,,$(VERSION))))
MINOR := $(word 2,$(subst ., ,$(VERSION)))
PATCH := $(word 3,$(subst ., ,$(VERSION)))

patch:
	$(eval NEW := v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))))
	@echo "$(VERSION) -> $(NEW)"
	git tag -a $(NEW) -m "Release $(NEW)"
	git push origin $(NEW)

minor:
	$(eval NEW := v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0)
	@echo "$(VERSION) -> $(NEW)"
	git tag -a $(NEW) -m "Release $(NEW)"
	git push origin $(NEW)

major:
	$(eval NEW := v$(shell echo $$(($(MAJOR)+1))).0.0)
	@echo "$(VERSION) -> $(NEW)"
	git tag -a $(NEW) -m "Release $(NEW)"
	git push origin $(NEW)
