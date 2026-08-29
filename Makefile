SHELL := /usr/bin/env bash

.PHONY: check ci inventory repository-check

check:
	golib check --all

ci: repository-check check

inventory repository-check:
	golib repository check
