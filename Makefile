.PHONY: install run

install:
	go install .
	ttyrant install-hooks

run:
	go build -o ttyrant . && ./ttyrant
